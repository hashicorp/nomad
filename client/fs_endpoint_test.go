// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// tempAllocDir returns a new alloc dir that is rooted in a temp dir. Caller
// should cleanup with AllocDir.Destroy()
func tempAllocDir(t testing.TB) *allocdir.AllocDir {
	dir := t.TempDir()

	require.NoError(t, os.Chmod(dir, 0o777))

	return allocdir.NewAllocDir(testlog.HCLogger(t), dir, "test_allocid")
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func TestFS_Stat_NoAlloc(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a client
	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Make the request with bad allocation id
	req := &cstructs.FsStatRequest{
		AllocID:      uuid.Generate(),
		Path:         "foo",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsStatResponse
	err := c.ClientRPC("FileSystem.Stat", req, &resp)
	require.NotNil(err)
	require.True(structs.IsErrUnknownAllocation(err))
}

func TestFS_Stat(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	// Create and add an alloc
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}
	// Wait for alloc to be running
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsStatRequest{
		AllocID:      alloc.ID,
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
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)[0]

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
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			req := &cstructs.FsStatRequest{
				AllocID: alloc.ID,
				Path:    "/",
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: structs.DefaultNamespace,
				},
			}

			var resp cstructs.FsStatResponse
			err := client.ClientRPC("FileSystem.Stat", req, &resp)
			if c.ExpectedError == "" {
				require.NoError(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), c.ExpectedError)
			}
		})
	}
}

func TestFS_List_NoAlloc(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a client
	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Make the request with bad allocation id
	req := &cstructs.FsListRequest{
		AllocID:      uuid.Generate(),
		Path:         "foo",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp cstructs.FsListResponse
	err := c.ClientRPC("FileSystem.List", req, &resp)
	require.NotNil(err)
	require.True(structs.IsErrUnknownAllocation(err))
}

func TestFS_List(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	// Create and add an alloc
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}
	// Wait for alloc to be running
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsListRequest{
		AllocID:      alloc.ID,
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
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)[0]

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
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsListRequest{
				AllocID: alloc.ID,
				Path:    "/",
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: structs.DefaultNamespace,
				},
			}

			var resp cstructs.FsListResponse
			err := client.ClientRPC("FileSystem.List", req, &resp)
			if c.ExpectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, c.ExpectedError)
			}
		})
	}
}

func TestFS_Stream_NoAlloc(t *testing.T) {
	ci.Parallel(t)
	ci.SkipSlow(t, "flaky on GHA; #12358")
	require := require.New(t)

	// Start a client
	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Make the request with bad allocation id
	req := &cstructs.FsStreamRequest{
		AllocID:      uuid.Generate(),
		Path:         "foo",
		Origin:       "start",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Stream")
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

			if structs.IsErrUnknownAllocation(msg.Error) {
				break OUTER
			} else {
				t.Fatalf("bad error: %v", err)
			}
		}
	}
}

// TestFS_Stream_GC asserts that reading files from an alloc that has been
// GC'ed from the client returns a 404 error.
func TestFS_Stream_GC(t *testing.T) {
	ci.Parallel(t)

	// Start a server and client.
	s, cleanupS := nomad.TestServer(t, nil)
	t.Cleanup(cleanupS)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { cleanupC() })

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Wait for alloc to be running.
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// GC alloc from the client.
	ar, err := c.getAllocRunner(alloc.ID)
	must.NoError(t, err)

	c.garbageCollector.MarkForCollection(alloc.ID, ar)
	must.True(t, c.CollectAllocation(alloc.ID))

	// Build the request.
	req := &cstructs.FsStreamRequest{
		AllocID:      alloc.ID,
		Path:         "alloc/logs/web.stdout.0",
		PlainText:    true,
		Follow:       true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler.
	handler, err := c.StreamingRpcHandler("FileSystem.Stream")
	must.NoError(t, err)

	// Create a pipe.
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler.
	go handler(p2)

	// Start the decoder.
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
	must.NoError(t, encoder.Encode(req))

	for {
		select {
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatal(err)
		case msg := <-streamMsg:
			must.Error(t, msg.Error)
			must.ErrorContains(t, msg.Error, "not found on client")
			must.Eq(t, http.StatusNotFound, *msg.Error.Code)
			return
		}
	}
}

func TestFS_Stream_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)[0]

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
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsStreamRequest{
				AllocID: alloc.ID,
				Path:    "foo",
				Origin:  "start",
				QueryOptions: structs.QueryOptions{
					Namespace: structs.DefaultNamespace,
					Region:    "global",
					AuthToken: c.Token,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("FileSystem.Stream")
			require.Nil(t, err)

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
						errCh <- err
						return
					}

					streamMsg <- &msg
				}
			}()

			// Send the request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			require.NoError(t, encoder.Encode(req))

			timeout := time.After(5 * time.Second)

		OUTER:
			for {
				select {
				case <-timeout:
					t.Fatal("timeout")
				case err := <-errCh:
					eof := err == io.EOF || strings.Contains(err.Error(), "closed")
					if c.ExpectedError == "" && eof {
						// No error was expected!
						return
					}
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

func TestFS_Stream(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	expected := "Hello from the other side"
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": expected,
	}

	// Wait for alloc to be running
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsStreamRequest{
		AllocID:      alloc.ID,
		Path:         "alloc/logs/web.stdout.0",
		PlainText:    true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Stream")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	// Wrap the pipe so we can check it is closed
	pipeChecker := &ReadWriteCloseChecker{ReadWriteCloser: p2}

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler
	go handler(pipeChecker)

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

	testutil.WaitForResult(func() (bool, error) {
		pipeChecker.l.Lock()
		defer pipeChecker.l.Unlock()

		return pipeChecker.Closed, nil
	}, func(err error) {
		t.Fatal("Pipe not closed")
	})
}

type ReadWriteCloseChecker struct {
	io.ReadWriteCloser
	l      sync.Mutex
	Closed bool
}

func (r *ReadWriteCloseChecker) Close() error {
	r.l.Lock()
	r.Closed = true
	r.l.Unlock()
	return r.ReadWriteCloser.Close()
}

func TestFS_Stream_Follow(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	expectedBase := "Hello from the other side"
	repeat := 10

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":                "20s",
		"stdout_string":          expectedBase,
		"stdout_repeat":          repeat,
		"stdout_repeat_duration": "200ms",
	}

	// Wait for alloc to be running
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsStreamRequest{
		AllocID:      alloc.ID,
		Path:         "alloc/logs/web.stdout.0",
		PlainText:    true,
		Follow:       true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Stream")
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

func TestFS_Stream_Limit(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	var limit int64 = 5
	full := "Hello from the other side"
	expected := full[:limit]
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": full,
	}

	// Wait for alloc to be running
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsStreamRequest{
		AllocID:      alloc.ID,
		Path:         "alloc/logs/web.stdout.0",
		PlainText:    true,
		Limit:        limit,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Stream")
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

func TestFS_Logs_NoAlloc(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a client
	c, cleanup := TestClient(t, nil)
	defer cleanup()

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

			if structs.IsErrUnknownAllocation(msg.Error) {
				break OUTER
			} else {
				t.Fatalf("bad error: %v", err)
			}
		}
	}
}

// TestFS_Logs_TaskPending asserts that trying to stream logs for tasks which
// have not started returns a 404 error.
func TestFS_Logs_TaskPending(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"start_block_for": "10s",
	}

	// Register job
	args := &structs.JobRegisterRequest{}
	args.Job = job
	args.WriteRequest.Region = "global"
	args.Namespace = job.Namespace
	var jobResp structs.JobRegisterResponse
	require.NoError(s.RPC("Job.Register", args, &jobResp))

	// Get the allocation ID
	var allocID string
	testutil.WaitForResult(func() (bool, error) {
		args := structs.AllocListRequest{}
		args.Region = "global"
		resp := structs.AllocListResponse{}
		if err := s.RPC("Alloc.List", &args, &resp); err != nil {
			return false, err
		}

		if len(resp.Allocations) != 1 {
			return false, fmt.Errorf("expected 1 alloc, found %d", len(resp.Allocations))
		}

		allocID = resp.Allocations[0].ID

		// wait for alloc runner to be created; otherwise, we get no alloc found error
		if _, err := c.getAllocRunner(allocID); err != nil {
			return false, fmt.Errorf("alloc runner was not created yet for %v", allocID)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("error getting alloc id: %v", err)
	})

	// Make the request
	req := &cstructs.FsLogsRequest{
		AllocID:      allocID,
		Task:         job.TaskGroups[0].Tasks[0].Name,
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

	for {
		select {
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatalf("unexpected stream error: %v", err)
		case msg := <-streamMsg:
			require.NotNil(msg.Error)
			require.NotNil(msg.Error.Code)
			require.EqualValues(http.StatusNotFound, *msg.Error.Code)
			require.Contains(msg.Error.Message, "not started")
			return
		}
	}
}

// TestFS_Logs_GC asserts that reading logs from an alloc that has been GC'ed
// from the client returns a 404 error.
func TestFS_Logs_GC(t *testing.T) {
	ci.Parallel(t)

	// Start a server and client.
	s, cleanupS := nomad.TestServer(t, nil)
	t.Cleanup(cleanupS)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	t.Cleanup(func() { cleanupC() })

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Wait for alloc to be running.
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// GC alloc from the client.
	ar, err := c.getAllocRunner(alloc.ID)
	must.NoError(t, err)

	c.garbageCollector.MarkForCollection(alloc.ID, ar)
	must.True(t, c.CollectAllocation(alloc.ID))

	// Build the request.
	req := &cstructs.FsLogsRequest{
		AllocID:      alloc.ID,
		Task:         job.TaskGroups[0].Tasks[0].Name,
		LogType:      "stdout",
		Origin:       "start",
		PlainText:    true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler.
	handler, err := c.StreamingRpcHandler("FileSystem.Logs")
	must.NoError(t, err)

	// Create a pipe.
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	// Start the handler.
	go handler(p2)

	// Start the decoder.
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

	// Send the request.
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	must.NoError(t, encoder.Encode(req))

	for {
		select {
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		case err := <-errCh:
			t.Fatalf("unexpected stream error: %v", err)
		case msg := <-streamMsg:
			must.Error(t, msg.Error)
			must.ErrorContains(t, msg.Error, "not found on client")
			must.Eq(t, http.StatusNotFound, *msg.Error.Code)
			return
		}
	}
}

func TestFS_Logs_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityReadLogs, acl.NamespaceCapabilityReadFS})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)[0]

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
			Name:  "good token",
			Token: tokenGood.SecretID,
		},
		{
			Name:  "root token",
			Token: root.SecretID,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Make the request with bad allocation id
			req := &cstructs.FsLogsRequest{
				AllocID: alloc.ID,
				Task:    job.TaskGroups[0].Tasks[0].Name,
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
						errCh <- err
						return
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
					eof := err == io.EOF || strings.Contains(err.Error(), "closed")
					if c.ExpectedError == "" && eof {
						// No error was expected!
						return
					}
					t.Fatal(err)
				case msg := <-streamMsg:
					if msg.Error == nil {
						continue
					}

					if strings.Contains(msg.Error.Error(), c.ExpectedError) {
						// Ok! Error matched expectation.
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
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	expected := "Hello from the other side\n"
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": expected,
	}

	// Wait for client to be running job
	testutil.WaitForRunning(t, s.RPC, job)

	// Get the allocation ID
	args := structs.AllocListRequest{}
	args.Region = "global"
	resp := structs.AllocListResponse{}
	require.NoError(s.RPC("Alloc.List", &args, &resp))
	require.Len(resp.Allocations, 1)
	allocID := resp.Allocations[0].ID

	// Make the request
	req := &cstructs.FsLogsRequest{
		AllocID:      allocID,
		Task:         job.TaskGroups[0].Tasks[0].Name,
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
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	expectedBase := "Hello from the other side\n"
	repeat := 10

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":                "20s",
		"stdout_string":          expectedBase,
		"stdout_repeat":          repeat,
		"stdout_repeat_duration": "200ms",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunning(t, s.RPC, job)[0]

	// Make the request
	req := &cstructs.FsLogsRequest{
		AllocID:      alloc.ID,
		Task:         job.TaskGroups[0].Tasks[0].Name,
		LogType:      "stdout",
		Origin:       "start",
		PlainText:    true,
		Follow:       true,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("FileSystem.Logs")
	require.NoError(err)

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

func TestFS_streamFile_NoFile(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	ad := tempAllocDir(t)
	defer ad.Destroy()

	frames := make(chan *sframer.StreamFrame, 32)
	framer := sframer.NewStreamFramer(frames, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	err := c.endpoints.FileSystem.streamFile(
		context.Background(), 0, "foo", 0, ad, framer, nil, false)
	require.Error(t, err)
	if runtime.GOOS == "windows" {
		require.Contains(t, err.Error(), "cannot find the file")
	} else {
		require.Contains(t, err.Error(), "no such file")
	}
}

func TestFS_streamFile_Modify(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Get a temp alloc dir
	ad := tempAllocDir(t)
	require.NoError(t, ad.Build())
	defer ad.Destroy()

	// Create a file in the temp dir
	streamFile := "stream_file"
	f, err := os.Create(filepath.Join(ad.AllocDir, streamFile))
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer f.Close()

	data := []byte("helloworld")

	// Start the reader
	resultCh := make(chan struct{})
	frames := make(chan *sframer.StreamFrame, 4)
	go func() {
		var collected []byte
		for {
			frame := <-frames
			if frame.IsHeartbeat() {
				continue
			}

			collected = append(collected, frame.Data...)
			if reflect.DeepEqual(data, collected) {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	// Write a few bytes
	if _, err := f.Write(data[:3]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	framer := sframer.NewStreamFramer(frames, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	// Start streaming
	go func() {
		if err := c.endpoints.FileSystem.streamFile(
			context.Background(), 0, streamFile, 0, ad, framer, nil, false); err != nil {
			t.Fatalf("stream() failed: %v", err)
		}
	}()

	// Sleep a little before writing more. This lets us check if the watch
	// is working.
	time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
	if _, err := f.Write(data[3:]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("failed to send new data")
	}
}

func TestFS_streamFile_Truncate(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Get a temp alloc dir
	ad := tempAllocDir(t)
	require.NoError(t, ad.Build())
	defer ad.Destroy()

	// Create a file in the temp dir
	data := []byte("helloworld")
	streamFile := "stream_file"
	streamFilePath := filepath.Join(ad.AllocDir, streamFile)
	f, err := os.Create(streamFilePath)
	require.NoError(t, err)
	defer f.Close()

	// Start the reader
	truncateCh := make(chan struct{})
	truncateClosed := false
	dataPostTruncCh := make(chan struct{})
	frames := make(chan *sframer.StreamFrame, 4)
	go func() {
		var collected []byte
		for {
			frame := <-frames
			if frame.IsHeartbeat() {
				continue
			}

			if frame.FileEvent == truncateEvent && !truncateClosed {
				close(truncateCh)
				truncateClosed = true
			}

			collected = append(collected, frame.Data...)
			if reflect.DeepEqual(data, collected) {
				close(dataPostTruncCh)
				return
			}
		}
	}()

	// Write a few bytes
	if _, err := f.Write(data[:3]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	framer := sframer.NewStreamFramer(frames, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	// Start streaming
	go func() {
		if err := c.endpoints.FileSystem.streamFile(
			context.Background(), 0, streamFile, 0, ad, framer, nil, false); err != nil {
			t.Fatalf("stream() failed: %v", err)
		}
	}()

	// Sleep a little before truncating. This lets us check if the watch
	// is working.
	time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
	if err := f.Truncate(0); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	f2, err := os.OpenFile(streamFilePath, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("failed to reopen file: %v", err)
	}
	defer f2.Close()
	if _, err := f2.Write(data[3:5]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case <-truncateCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive truncate")
	}

	// Sleep a little before writing more. This lets us check if the watch
	// is working.
	time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
	if _, err := f2.Write(data[5:]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	select {
	case <-dataPostTruncCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive post truncate data")
	}
}

func TestFS_streamImpl_Delete(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not allow us to delete a file while it is open")
	}

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Get a temp alloc dir
	ad := tempAllocDir(t)
	require.NoError(t, ad.Build())
	defer ad.Destroy()

	// Create a file in the temp dir
	data := []byte("helloworld")
	streamFile := "stream_file"
	streamFilePath := filepath.Join(ad.AllocDir, streamFile)
	f, err := os.Create(streamFilePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer f.Close()

	// Start the reader
	deleteCh := make(chan struct{})
	frames := make(chan *sframer.StreamFrame, 4)
	go func() {
		for {
			frame, ok := <-frames
			if !ok {
				return
			}

			if frame.IsHeartbeat() {
				continue
			}

			if frame.FileEvent == deleteEvent {
				close(deleteCh)
				return
			}
		}
	}()

	// Write a few bytes
	if _, err := f.Write(data[:3]); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	framer := sframer.NewStreamFramer(frames, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	// Start streaming
	go func() {
		if err := c.endpoints.FileSystem.streamFile(
			context.Background(), 0, streamFile, 0, ad, framer, nil, false); err != nil {
			t.Fatalf("stream() failed: %v", err)
		}
	}()

	// Sleep a little before deleting. This lets us check if the watch
	// is working.
	time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
	if err := os.Remove(streamFilePath); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	select {
	case <-deleteCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive delete")
	}
}

func TestFS_logsImpl_NoFollow(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Get a temp alloc dir and create the log dir
	ad := tempAllocDir(t)
	require.NoError(t, ad.Build())
	defer ad.Destroy()

	logDir := filepath.Join(ad.SharedDir, allocdir.LogDirName)
	if err := os.MkdirAll(logDir, 0777); err != nil {
		t.Fatalf("Failed to make log dir: %v", err)
	}

	// Create a series of log files in the temp dir
	task := "foo"
	logType := "stdout"
	expected := []byte("012")
	for i := 0; i < 3; i++ {
		logFile := fmt.Sprintf("%s.%s.%d", task, logType, i)
		logFilePath := filepath.Join(logDir, logFile)
		err := os.WriteFile(logFilePath, expected[i:i+1], 0777)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Start the reader
	resultCh := make(chan struct{})
	frames := make(chan *sframer.StreamFrame, 4)
	var received []byte
	go func() {
		for {
			frame, ok := <-frames
			if !ok {
				return
			}

			if frame.IsHeartbeat() {
				continue
			}

			received = append(received, frame.Data...)
			if reflect.DeepEqual(received, expected) {
				close(resultCh)
				return
			}
		}
	}()

	// Start streaming logs
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := c.endpoints.FileSystem.logsImpl(
		ctx, false, false, 0,
		OriginStart, task, logType, ad, frames); err != nil {
		t.Fatalf("logsImpl failed: %v", err)
	}

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive data: got %q", string(received))
	}
}

func TestFS_logsImpl_Follow(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Get a temp alloc dir and create the log dir
	ad := tempAllocDir(t)
	require.NoError(t, ad.Build())
	defer ad.Destroy()

	logDir := filepath.Join(ad.SharedDir, allocdir.LogDirName)
	if err := os.MkdirAll(logDir, 0777); err != nil {
		t.Fatalf("Failed to make log dir: %v", err)
	}

	// Create a series of log files in the temp dir
	task := "foo"
	logType := "stdout"
	expected := []byte("012345")
	initialWrites := 3

	filePath := func(index int) string {
		logFile := fmt.Sprintf("%s.%s.%d", task, logType, index)
		return filepath.Join(logDir, logFile)
	}
	writeToFile := func(index int, data []byte) {
		logFilePath := filePath(index)
		err := os.WriteFile(logFilePath, data, 0777)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}
	appendToFile := func(index int, data []byte) {
		logFilePath := filePath(index)
		f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		defer f.Close()

		if _, err = f.Write(data); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}
	for i := 0; i < initialWrites; i++ {
		writeToFile(i, expected[i:i+1])
	}

	// Start the reader
	firstResultCh := make(chan struct{})
	fullResultCh := make(chan struct{})
	frames := make(chan *sframer.StreamFrame, 4)
	var received []byte
	go func() {
		for {
			frame, ok := <-frames
			if !ok {
				return
			}

			if frame.IsHeartbeat() {
				continue
			}

			received = append(received, frame.Data...)
			if reflect.DeepEqual(received, expected[:initialWrites]) {
				close(firstResultCh)
			} else if reflect.DeepEqual(received, expected) {
				close(fullResultCh)
				return
			}
		}
	}()

	// Start streaming logs
	go c.endpoints.FileSystem.logsImpl(
		context.Background(), true, false, 0,
		OriginStart, task, logType, ad, frames)

	select {
	case <-firstResultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive data: got %q", string(received))
	}

	// We got the first chunk of data, write out the rest splitted
	// between the last file and to the next file
	// at an index much ahead to check that it is following and detecting
	// skips
	skipTo := initialWrites + 10
	appendToFile(initialWrites-1, expected[initialWrites:initialWrites+1])
	writeToFile(skipTo, expected[initialWrites+1:])

	select {
	case <-fullResultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
		t.Fatalf("did not receive data: got %q", string(received))
	}
}
