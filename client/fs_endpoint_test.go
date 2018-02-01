package client

import (
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestFS_Stat_NoAlloc(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a client
	c := TestClient(t, nil)
	defer c.Shutdown()

	// Make the request with bad allocation id
	req := &cstructs.FsStatRequest{
		AllocID:      uuid.Generate(),
		Path:         "foo",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsStatResponse
	err := c.ClientRPC("FileSystem.Stat", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "unknown")
}

func TestFS_Stat(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a client
	c := TestClient(t, nil)
	defer c.Shutdown()

	// Create and add an alloc
	a := mock.Alloc()
	c.addAlloc(a, "")

	// Wait for the client to start it
	testutil.WaitForResult(func() (bool, error) {
		ar, ok := c.allocs[a.ID]
		if !ok {
			return false, fmt.Errorf("alloc doesn't exist")
		}

		return len(ar.tasks) != 0, fmt.Errorf("tasks not running")
	}, func(err error) {
		t.Fatal(err)
	})

	// Make the request with bad allocation id
	req := &cstructs.FsStatRequest{
		AllocID:      a.ID,
		Path:         "/",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsStatResponse
	err := c.ClientRPC("FileSystem.Stat", req, &resp)
	require.Nil(err)
	require.NotNil(resp.Info)
	require.True(resp.Info.IsDir)
}

func TestFS_Stat_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server
	s, root := nomad.TestACLServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.RPC)

	client := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer client.Shutdown()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "unknown allocation",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "unknown allocation",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsStatRequest{
				AllocID: uuid.Generate(),
				Path:    "/",
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: structs.DefaultNamespace,
				},
			}

			var resp cstructs.FsStatResponse
			err := client.ClientRPC("FileSystem.Stat", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestFS_List_NoAlloc(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a client
	c := TestClient(t, nil)
	defer c.Shutdown()

	// Make the request with bad allocation id
	req := &cstructs.FsListRequest{
		AllocID:      uuid.Generate(),
		Path:         "foo",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsListResponse
	err := c.ClientRPC("FileSystem.List", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "unknown")
}

func TestFS_List(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a client
	c := TestClient(t, nil)
	defer c.Shutdown()

	// Create and add an alloc
	a := mock.Alloc()
	c.addAlloc(a, "")

	// Wait for the client to start it
	testutil.WaitForResult(func() (bool, error) {
		ar, ok := c.allocs[a.ID]
		if !ok {
			return false, fmt.Errorf("alloc doesn't exist")
		}

		return len(ar.tasks) != 0, fmt.Errorf("tasks not running")
	}, func(err error) {
		t.Fatal(err)
	})

	// Make the request
	req := &cstructs.FsListRequest{
		AllocID:      a.ID,
		Path:         "/",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsListResponse
	err := c.ClientRPC("FileSystem.List", req, &resp)
	require.Nil(err)
	require.NotEmpty(resp.Files)
	require.True(resp.Files[0].IsDir)
}

func TestFS_List_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server
	s, root := nomad.TestACLServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.RPC)

	client := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer client.Shutdown()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "unknown allocation",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "unknown allocation",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsListRequest{
				AllocID: uuid.Generate(),
				Path:    "/",
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: structs.DefaultNamespace,
				},
			}

			var resp cstructs.FsListResponse
			err := client.ClientRPC("FileSystem.List", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestFS_Logs_NoAlloc(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a client
	c := TestClient(t, nil)
	defer c.Shutdown()

	// Make the request with bad allocation id
	req := &cstructs.FsLogsRequest{
		AllocID:      uuid.Generate(),
		Task:         "foo",
		LogType:      "stdout",
		Origin:       "start",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Logs")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler
	go handler(p2)

	// Start the decoder
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg cstructs.StreamErrWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %v", err)
			}

			streamMsg <- &msg
		}
	}()

	// Send the request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(3 * time.Second)

OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			t.Logf("Got msg %+v", msg)
			if msg.Error == nil {
				continue
			}

			if strings.Contains(msg.Error.Error(), "unknown alloc") {
				break OUTER
			} else {
				t.Fatalf("bad error: %v", err)
			}
		}
	}
}

func TestFS_Logs_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server
	s, root := nomad.TestACLServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.RPC)

	client := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer client.Shutdown()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "unknown allocation",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "unknown allocation",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsLogsRequest{
				AllocID: uuid.Generate(),
				Task:    "foo",
				LogType: "stdout",
				Origin:  "start",
				QueryOptions: structs.QueryOptions{
					Namespace: structs.DefaultNamespace,
					Region:    "global",
					AuthToken: c.Token,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("FileSystem.Logs")
			require.Nil(err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			streamMsg := make(chan *cstructs.StreamErrWrapper)

			// Start the handler
			go handler(p2)

			// Start the decoder
			go func() {
				decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
				for {
					var msg cstructs.StreamErrWrapper
					if err := decoder.Decode(&msg); err != nil {
						if err == io.EOF || strings.Contains(err.Error(), "closed") {
							return
						}
						errCh <- fmt.Errorf("error decoding: %v", err)
					}

					streamMsg <- &msg
				}
			}()

			// Send the request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			require.Nil(encoder.Encode(req))

			timeout := time.After(5 * time.Second)

		OUTER:
			for {
				select {
				case <-timeout:
					t.Fatal("timeout")
				case err := <-errCh:
					t.Fatal(err)
				case msg := <-streamMsg:
					if msg.Error == nil {
						continue
					}

					if strings.Contains(msg.Error.Error(), c.ExpectedError) {
						break OUTER
					} else {
						t.Fatalf("Bad error: %v", msg.Error)
					}
				}
			}
		})
	}
}

func TestFS_Logs(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s := nomad.TestServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.RPC)

	c := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer c.Shutdown()

	// Force an allocation onto the node
	expected := "Hello from the other side"
	a := mock.Alloc()
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for":       "2s",
			"stdout_string": expected,
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		node, err := s.State().NodeByID(nil, c.NodeID())
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, fmt.Errorf("unknown node")
		}

		return node.Status == structs.NodeStatusReady, fmt.Errorf("bad node status")
	}, func(err error) {
		t.Fatal(err)
	})

	// Upsert the allocation
	state := s.State()
	require.Nil(state.UpsertJob(999, a.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Make the request
	req := &cstructs.FsLogsRequest{
		AllocID:      a.ID,
		Task:         a.Job.TaskGroups[0].Tasks[0].Name,
		LogType:      "stdout",
		Origin:       "start",
		PlainText:    true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Logs")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler
	go handler(p2)

	// Start the decoder
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg cstructs.StreamErrWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %v", err)
			}

			streamMsg <- &msg
		}
	}()

	// Send the request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(3 * time.Second)
	received := ""
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			if msg.Error != nil {
				t.Fatalf("Got error: %v", msg.Error.Error())
			}

			// Add the payload
			received += string(msg.Payload)
			if received == expected {
				break OUTER
			}
		}
	}
}

func TestFS_Logs_Follow(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s := nomad.TestServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.RPC)

	c := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer c.Shutdown()

	// Force an allocation onto the node
	expectedBase := "Hello from the other side"
	repeat := 10

	a := mock.Alloc()
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for":                "20s",
			"stdout_string":          expectedBase,
			"stdout_repeat":          repeat,
			"stdout_repeat_duration": 200 * time.Millisecond,
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		node, err := s.State().NodeByID(nil, c.NodeID())
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, fmt.Errorf("unknown node")
		}

		return node.Status == structs.NodeStatusReady, fmt.Errorf("bad node status")
	}, func(err error) {
		t.Fatal(err)
	})

	// Upsert the allocation
	state := s.State()
	require.Nil(state.UpsertJob(999, a.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})

	// Make the request
	req := &cstructs.FsLogsRequest{
		AllocID:      a.ID,
		Task:         a.Job.TaskGroups[0].Tasks[0].Name,
		LogType:      "stdout",
		Origin:       "start",
		PlainText:    true,
		Follow:       true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := s.StreamingRpcHandler("FileSystem.Logs")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler
	go handler(p2)

	// Start the decoder
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg cstructs.StreamErrWrapper
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "closed") {
					return
				}
				errCh <- fmt.Errorf("error decoding: %v", err)
			}

			streamMsg <- &msg
		}
	}()

	// Send the request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(20 * time.Second)
	expected := strings.Repeat(expectedBase, repeat+1)
	received := ""
OUTER:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			if msg.Error != nil {
				t.Fatalf("Got error: %v", msg.Error.Error())
			}

			// Add the payload
			received += string(msg.Payload)
			if received == expected {
				break OUTER
			}
		}
	}
}

func TestFS_findClosest(t *testing.T) {
	task := "foo"
	entries := []*cstructs.AllocFileInfo{
		{
			Name: "foo.stdout.0",
			Size: 100,
		},
		{
			Name: "foo.stdout.1",
			Size: 100,
		},
		{
			Name: "foo.stdout.2",
			Size: 100,
		},
		{
			Name: "foo.stdout.3",
			Size: 100,
		},
		{
			Name: "foo.stderr.0",
			Size: 100,
		},
		{
			Name: "foo.stderr.1",
			Size: 100,
		},
		{
			Name: "foo.stderr.2",
			Size: 100,
		},
	}

	cases := []struct {
		Entries        []*cstructs.AllocFileInfo
		DesiredIdx     int64
		DesiredOffset  int64
		Task           string
		LogType        string
		ExpectedFile   string
		ExpectedIdx    int64
		ExpectedOffset int64
		Error          bool
	}{
		// Test error cases
		{
			Entries:    nil,
			DesiredIdx: 0,
			Task:       task,
			LogType:    "stdout",
			Error:      true,
		},
		{
			Entries:    entries[0:3],
			DesiredIdx: 0,
			Task:       task,
			LogType:    "stderr",
			Error:      true,
		},

		// Test beginning cases
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[0].Name,
			ExpectedIdx:  0,
		},
		{
			// Desired offset should be ignored at edges
			Entries:        entries,
			DesiredIdx:     0,
			DesiredOffset:  -100,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[0].Name,
			ExpectedIdx:    0,
			ExpectedOffset: 0,
		},
		{
			// Desired offset should be ignored at edges
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  -1000,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[0].Name,
			ExpectedIdx:    0,
			ExpectedOffset: 0,
		},
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[4].Name,
			ExpectedIdx:  0,
		},
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[0].Name,
			ExpectedIdx:  0,
		},

		// Test middle cases
		{
			Entries:      entries,
			DesiredIdx:   1,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[1].Name,
			ExpectedIdx:  1,
		},
		{
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  10,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[1].Name,
			ExpectedIdx:    1,
			ExpectedOffset: 10,
		},
		{
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  110,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[2].Name,
			ExpectedIdx:    2,
			ExpectedOffset: 10,
		},
		{
			Entries:      entries,
			DesiredIdx:   1,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[5].Name,
			ExpectedIdx:  1,
		},
		// Test end cases
		{
			Entries:      entries,
			DesiredIdx:   math.MaxInt64,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[3].Name,
			ExpectedIdx:  3,
		},
		{
			Entries:        entries,
			DesiredIdx:     math.MaxInt64,
			DesiredOffset:  math.MaxInt64,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[3].Name,
			ExpectedIdx:    3,
			ExpectedOffset: 100,
		},
		{
			Entries:        entries,
			DesiredIdx:     math.MaxInt64,
			DesiredOffset:  -10,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[3].Name,
			ExpectedIdx:    3,
			ExpectedOffset: 90,
		},
		{
			Entries:      entries,
			DesiredIdx:   math.MaxInt64,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[6].Name,
			ExpectedIdx:  2,
		},
	}

	for i, c := range cases {
		entry, idx, offset, err := findClosest(c.Entries, c.DesiredIdx, c.DesiredOffset, c.Task, c.LogType)
		if err != nil {
			if !c.Error {
				t.Fatalf("case %d: Unexpected error: %v", i, err)
			}
			continue
		}

		if entry.Name != c.ExpectedFile {
			t.Fatalf("case %d: Got file %q; want %q", i, entry.Name, c.ExpectedFile)
		}
		if idx != c.ExpectedIdx {
			t.Fatalf("case %d: Got index %d; want %d", i, idx, c.ExpectedIdx)
		}
		if offset != c.ExpectedOffset {
			t.Fatalf("case %d: Got offset %d; want %d", i, offset, c.ExpectedOffset)
		}
	}
}
