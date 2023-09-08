// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/lib/proclib"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestAllocations_Restart(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	a := mock.Alloc()
	a.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	a.Job.TaskGroups[0].RestartPolicy = &nstructs.RestartPolicy{
		Attempts: 0,
		Mode:     nstructs.RestartPolicyModeFail,
	}
	a.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}
	require.Nil(client.addAlloc(a, ""))

	// Try with bad alloc
	req := &nstructs.AllocRestartRequest{}
	var resp nstructs.GenericResponse
	err := client.ClientRPC("Allocations.Restart", &req, &resp)
	require.Error(err)

	// Try with good alloc
	req.AllocID = a.ID

	testutil.WaitForResult(func() (bool, error) {
		var resp2 nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp2)
		if err != nil && strings.Contains(err.Error(), "not running") {
			return false, err
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_RestartAllTasks(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	alloc := mock.LifecycleAlloc()
	require.Nil(client.addAlloc(alloc, ""))

	// setup process wranglers for our tasks to make sure they work with restart
	client.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "web"})
	client.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "init"})
	client.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "side"})
	client.wranglers.Setup(proclib.Task{AllocID: alloc.ID, Task: "poststart"})

	// Can't restart all tasks while specifying a task name.
	req := &nstructs.AllocRestartRequest{
		AllocID:  alloc.ID,
		AllTasks: true,
		TaskName: "web",
	}
	var resp nstructs.GenericResponse
	err := client.ClientRPC("Allocations.Restart", &req, &resp)
	require.Error(err)

	// Good request.
	req = &nstructs.AllocRestartRequest{
		AllocID:  alloc.ID,
		AllTasks: true,
	}

	testutil.WaitForResult(func() (bool, error) {
		var resp2 nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp2)
		if err != nil && strings.Contains(err.Error(), "not running") {
			return false, err
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_Restart_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, server.RPC, job, root.SecretID)[0]

	// Try request without a token and expect failure
	{
		req := &nstructs.AllocRestartRequest{}
		req.AllocID = alloc.ID
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp)
		require.NotNil(err)
		require.ErrorContains(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{}))
		req := &nstructs.AllocRestartRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		policyHCL := mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilityAllocLifecycle})
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", policyHCL)
		require.NotNil(token)
		req := &nstructs.AllocRestartRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp)
		require.NoError(err)
		//require.True(nstructs.IsErrUnknownAllocation(err), "Expected unknown alloc, found: %v", err)
	}

	// Try request with a management token
	{
		req := &nstructs.AllocRestartRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = root.SecretID
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Restart", &req, &resp)
		// Depending on how quickly the alloc restarts there may be no
		// error *or* a task not running error; either is fine.
		if err != nil {
			require.Contains(err.Error(), "Task not running", err)
		}
	}
}

func TestAllocations_GarbageCollectAll(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	req := &nstructs.NodeSpecificRequest{}
	var resp nstructs.GenericResponse
	require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
}

func TestAllocations_GarbageCollectAll_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	// Try request without a token and expect failure
	{
		req := &nstructs.NodeSpecificRequest{}
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID
		var resp nstructs.GenericResponse
		require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
	}

	// Try request with a management token
	{
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = root.SecretID
		var resp nstructs.GenericResponse
		require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
	}
}

func TestAllocations_GarbageCollect(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanup()

	a := mock.Alloc()
	a.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	rp := &nstructs.RestartPolicy{
		Attempts: 0,
		Mode:     nstructs.RestartPolicyModeFail,
	}
	a.Job.TaskGroups[0].RestartPolicy = rp
	a.Job.TaskGroups[0].Tasks[0].RestartPolicy = rp
	a.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10ms",
	}
	require.Nil(client.addAlloc(a, ""))

	// Try with bad alloc
	req := &nstructs.AllocSpecificRequest{}
	var resp nstructs.GenericResponse
	err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
	require.NotNil(err)

	// Try with good alloc
	req.AllocID = a.ID
	testutil.WaitForResult(func() (bool, error) {
		// Check if has been removed first
		if ar, ok := client.allocs[a.ID]; !ok || ar.IsDestroyed() {
			return true, nil
		}

		var resp2 nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp2)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_GarbageCollect_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	noSuchAllocErr := fmt.Errorf("No such allocation on client or allocation not eligible for GC")

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, server.RPC, job, root.SecretID)[0]

	// Try request without a token and expect failure
	{
		req := &nstructs.AllocSpecificRequest{}
		req.AllocID = alloc.ID
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.NotNil(err)
		require.ErrorContains(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.AllocSpecificRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "test-valid",
			mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
		req := &nstructs.AllocSpecificRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.Error(err, noSuchAllocErr)
	}

	// Try request with a management token
	{
		req := &nstructs.AllocSpecificRequest{}
		req.AuthToken = root.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.Error(err, noSuchAllocErr)
	}
}

func TestAllocations_Signal(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	a := mock.Alloc()
	require.Nil(t, client.addAlloc(a, ""))

	// Try with bad alloc
	req := &nstructs.AllocSignalRequest{}
	var resp nstructs.GenericResponse
	err := client.ClientRPC("Allocations.Signal", &req, &resp)
	require.NotNil(t, err)
	require.True(t, nstructs.IsErrUnknownAllocation(err))

	// Try with good alloc
	req.AllocID = a.ID

	var resp2 nstructs.GenericResponse
	err = client.ClientRPC("Allocations.Signal", &req, &resp2)

	require.Error(t, err, "Expected error, got: %s, resp: %#+v", err, resp2)
	require.Contains(t, err.Error(), "Failed to signal task: web, err: Task not running")
}

func TestAllocations_Signal_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, server.RPC, job, root.SecretID)[0]

	// Try request without a token and expect failure
	{
		req := &nstructs.AllocSignalRequest{}
		req.AllocID = alloc.ID
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Signal", &req, &resp)
		require.NotNil(err)
		require.ErrorContains(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.AllocSignalRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Signal", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "test-valid",
			mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilityAllocLifecycle}))
		req := &nstructs.AllocSignalRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Signal", &req, &resp)
		require.NoError(err)
	}

	// Try request with a management token
	{
		req := &nstructs.AllocSignalRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = root.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.Signal", &req, &resp)
		require.NoError(err)
	}
}

func TestAllocations_Stats(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	a := mock.Alloc()
	require.Nil(client.addAlloc(a, ""))

	// Try with bad alloc
	req := &cstructs.AllocStatsRequest{}
	var resp cstructs.AllocStatsResponse
	err := client.ClientRPC("Allocations.Stats", &req, &resp)
	require.NotNil(err)

	// Try with good alloc
	req.AllocID = a.ID
	testutil.WaitForResult(func() (bool, error) {
		var resp2 cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp2)
		if err != nil {
			return false, err
		}
		if resp2.Stats == nil {
			return false, fmt.Errorf("invalid stats object")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_Stats_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
	}

	// Wait for client to be running job
	alloc := testutil.WaitForRunningWithToken(t, server.RPC, job, root.SecretID)[0]

	// Try request without a token and expect failure
	{
		req := &cstructs.AllocStatsRequest{}
		req.AllocID = alloc.ID
		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.NotNil(err)
		require.ErrorContains(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &cstructs.AllocStatsRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "test-valid",
			mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req := &cstructs.AllocStatsRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.NoError(err)
	}

	// Try request with a management token
	{
		req := &cstructs.AllocStatsRequest{}
		req.AllocID = alloc.ID
		req.AuthToken = root.SecretID

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.NoError(err)
	}
}

func TestAlloc_Checks(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	t.Cleanup(func() {
		must.NoError(t, cleanup())
	})

	now := time.Date(2022, 3, 4, 5, 6, 7, 8, time.UTC).Unix()

	qr1 := &nstructs.CheckQueryResult{
		ID:        "abc123",
		Mode:      "healthiness",
		Status:    "passing",
		Output:    "nomad: http ok",
		Timestamp: now,
		Group:     "group",
		Task:      "task",
		Service:   "service",
		Check:     "check",
	}

	qr2 := &nstructs.CheckQueryResult{
		ID:        "def456",
		Mode:      "readiness",
		Status:    "passing",
		Output:    "nomad: http ok",
		Timestamp: now,
		Group:     "group",
		Service:   "service2",
		Check:     "check",
	}

	t.Run("alloc does not exist", func(t *testing.T) {
		request := cstructs.AllocChecksRequest{AllocID: "d3e34248-4843-be75-d4fd-4899975cfb38"}
		var response cstructs.AllocChecksResponse
		err := client.ClientRPC("Allocations.Checks", &request, &response)
		must.EqError(t, err, `Unknown allocation "d3e34248-4843-be75-d4fd-4899975cfb38"`)
	})

	t.Run("no checks for alloc", func(t *testing.T) {
		alloc := mock.Alloc()
		must.NoError(t, client.addAlloc(alloc, ""))

		request := cstructs.AllocChecksRequest{AllocID: alloc.ID}
		var response cstructs.AllocChecksResponse
		err := client.ClientRPC("Allocations.Checks", &request, &response)
		must.NoError(t, err)
		must.MapEmpty(t, response.Results)
	})

	t.Run("two in one alloc", func(t *testing.T) {
		alloc := mock.Alloc()
		must.NoError(t, client.addAlloc(alloc, ""))
		must.NoError(t, client.checkStore.Set(alloc.ID, qr1))
		must.NoError(t, client.checkStore.Set(alloc.ID, qr2))

		request := cstructs.AllocChecksRequest{AllocID: alloc.ID}
		var response cstructs.AllocChecksResponse
		err := client.ClientRPC("Allocations.Checks", &request, &response)
		must.NoError(t, err)
		must.MapEq(t, map[nstructs.CheckID]*nstructs.CheckQueryResult{
			"abc123": qr1,
			"def456": qr2,
		}, response.Results)
	})

	t.Run("ignore unrelated alloc", func(t *testing.T) {
		alloc1 := mock.Alloc()
		must.NoError(t, client.addAlloc(alloc1, ""))

		alloc2 := mock.Alloc()
		must.NoError(t, client.addAlloc(alloc2, ""))
		must.NoError(t, client.checkStore.Set(alloc1.ID, qr1))
		must.NoError(t, client.checkStore.Set(alloc2.ID, qr2))

		request := cstructs.AllocChecksRequest{AllocID: alloc1.ID}
		var response cstructs.AllocChecksResponse
		err := client.ClientRPC("Allocations.Checks", &request, &response)
		must.NoError(t, err)
		must.MapEq(t, map[nstructs.CheckID]*nstructs.CheckQueryResult{
			"abc123": qr1,
		}, response.Results)
	})
}

func TestAlloc_ExecStreaming(t *testing.T) {
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

	expectedStdout := "Hello from the other side\n"
	expectedStderr := "Hello from the other side\n"
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"stdout_string": expectedStdout,
			"stderr_string": expectedStderr,
			"exit_code":     3,
		},
	}

	// Wait for client to be running job
	testutil.WaitForRunning(t, s.RPC, job)

	// Get the allocation ID
	args := nstructs.AllocListRequest{}
	args.Region = "global"
	resp := nstructs.AllocListResponse{}
	require.NoError(s.RPC("Alloc.List", &args, &resp))
	require.Len(resp.Allocations, 1)
	allocID := resp.Allocations[0].ID

	// Make the request
	req := &cstructs.AllocExecRequest{
		AllocID:      allocID,
		Task:         job.TaskGroups[0].Tasks[0].Name,
		Tty:          true,
		Cmd:          []string{"placeholder command"},
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("Allocations.Exec")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

	// Start the handler
	go handler(p2)
	go decodeFrames(t, p1, frames, errCh)

	// Send the request
	encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(3 * time.Second)

	exitCode := -1
	receivedStdout := ""
	receivedStderr := ""

OUTER:
	for {
		select {
		case <-timeout:
			// time out report
			require.Equal(expectedStdout, receivedStderr, "didn't receive expected stdout")
			require.Equal(expectedStderr, receivedStderr, "didn't receive expected stderr")
			require.Equal(3, exitCode, "failed to get exit code")
			require.FailNow("timed out")
		case err := <-errCh:
			require.NoError(err)
		case f := <-frames:
			switch {
			case f.Stdout != nil && len(f.Stdout.Data) != 0:
				receivedStdout += string(f.Stdout.Data)
			case f.Stderr != nil && len(f.Stderr.Data) != 0:
				receivedStderr += string(f.Stderr.Data)
			case f.Exited && f.Result != nil:
				exitCode = int(f.Result.ExitCode)
			default:
				t.Logf("received unrelevant frame: %v", f)
			}

			if expectedStdout == receivedStdout && expectedStderr == receivedStderr && exitCode == 3 {
				break OUTER
			}
		}
	}
}

func TestAlloc_ExecStreaming_NoAllocation(t *testing.T) {
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

	// Make the request
	req := &cstructs.AllocExecRequest{
		AllocID:      uuid.Generate(),
		Task:         "testtask",
		Tty:          true,
		Cmd:          []string{"placeholder command"},
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("Allocations.Exec")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

	// Start the handler
	go handler(p2)
	go decodeFrames(t, p1, frames, errCh)

	// Send the request
	encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(3 * time.Second)

	select {
	case <-timeout:
		require.FailNow("timed out")
	case err := <-errCh:
		require.True(nstructs.IsErrUnknownAllocation(err), "expected no allocation error but found: %v", err)
	case f := <-frames:
		require.Fail("received unexpected frame", "frame: %#v", f)
	}
}

func TestAlloc_ExecStreaming_DisableRemoteExec(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
		c.DisableRemoteExec = true
	})
	defer cleanupC()

	// Make the request
	req := &cstructs.AllocExecRequest{
		AllocID:      uuid.Generate(),
		Task:         "testtask",
		Tty:          true,
		Cmd:          []string{"placeholder command"},
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Get the handler
	handler, err := c.StreamingRpcHandler("Allocations.Exec")
	require.Nil(err)

	// Create a pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

	// Start the handler
	go handler(p2)
	go decodeFrames(t, p1, frames, errCh)

	// Send the request
	encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
	require.Nil(encoder.Encode(req))

	timeout := time.After(3 * time.Second)

	select {
	case <-timeout:
		require.FailNow("timed out")
	case err := <-errCh:
		require.True(nstructs.IsErrPermissionDenied(err), "expected permission denied error but found: %v", err)
	case f := <-frames:
		require.Fail("received unexpected frame", "frame: %#v", f)
	}
}

func TestAlloc_ExecStreaming_ACL_Basic(t *testing.T) {
	ci.Parallel(t)

	// Start a server and client
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanupC()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec, acl.NamespaceCapabilityReadFS})
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
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "task not found",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "task not found",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request
			req := &cstructs.AllocExecRequest{
				AllocID: alloc.ID,
				Task:    "testtask",
				Tty:     true,
				Cmd:     []string{"placeholder command"},
				QueryOptions: nstructs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("Allocations.Exec")
			require.Nil(t, err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

			// Start the handler
			go handler(p2)
			go decodeFrames(t, p1, frames, errCh)

			// Send the request
			encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
			require.Nil(t, encoder.Encode(req))

			select {
			case <-time.After(3 * time.Second):
				require.FailNow(t, "timed out")
			case err := <-errCh:
				require.Contains(t, err.Error(), c.ExpectedError)
			case f := <-frames:
				require.Fail(t, "received unexpected frame", "frame: %#v", f)
			}
		})
	}
}

// TestAlloc_ExecStreaming_ACL_WithIsolation_Image asserts that token only needs
// alloc-exec acl policy when image isolation is used
func TestAlloc_ExecStreaming_ACL_WithIsolation_Image(t *testing.T) {
	ci.Parallel(t)
	isolation := drivers.FSIsolationImage

	// Start a server and client
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}

		pluginConfig := []*nconfig.PluginConfig{
			{
				Name: "mock_driver",
				Config: map[string]interface{}{
					"fs_isolation": string(isolation),
				},
			},
		}

		c.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", map[string]string{}, pluginConfig)
	})
	defer cleanupC()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyAllocExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec})
	tokenAllocExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyAllocExec)

	policyAllocNodeExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec, acl.NamespaceCapabilityAllocNodeExec})
	tokenAllocNodeExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyAllocNodeExec)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"stdout_string": "some output",
		},
	}

	// Wait for client to be running job
	testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)

	// Get the allocation ID
	args := nstructs.AllocListRequest{}
	args.Region = "global"
	args.AuthToken = root.SecretID
	args.Namespace = nstructs.DefaultNamespace
	resp := nstructs.AllocListResponse{}
	require.NoError(t, s.RPC("Alloc.List", &args, &resp))
	require.Len(t, resp.Allocations, 1)
	allocID := resp.Allocations[0].ID

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "alloc-exec token",
			Token:         tokenAllocExec.SecretID,
			ExpectedError: "",
		},
		{
			Name:          "alloc-node-exec token",
			Token:         tokenAllocNodeExec.SecretID,
			ExpectedError: "",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request
			req := &cstructs.AllocExecRequest{
				AllocID: allocID,
				Task:    job.TaskGroups[0].Tasks[0].Name,
				Tty:     true,
				Cmd:     []string{"placeholder command"},
				QueryOptions: nstructs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("Allocations.Exec")
			require.Nil(t, err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

			// Start the handler
			go handler(p2)
			go decodeFrames(t, p1, frames, errCh)

			// Send the request
			encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
			require.Nil(t, encoder.Encode(req))

			select {
			case <-time.After(3 * time.Second):
			case err := <-errCh:
				if c.ExpectedError == "" {
					require.NoError(t, err)
				} else {
					require.Contains(t, err.Error(), c.ExpectedError)
				}
			case f := <-frames:
				// we are good if we don't expect an error
				if c.ExpectedError != "" {
					require.Fail(t, "unexpected frame", "frame: %#v", f)
				}
			}
		})
	}
}

// TestAlloc_ExecStreaming_ACL_WithIsolation_Chroot asserts that token only needs
// alloc-exec acl policy when chroot isolation is used
func TestAlloc_ExecStreaming_ACL_WithIsolation_Chroot(t *testing.T) {
	ci.SkipSlow(t, "flaky on GHA; too much disk IO")
	ci.Parallel(t)

	if runtime.GOOS != "linux" || unix.Geteuid() != 0 {
		t.Skip("chroot isolation requires linux root")
	}

	isolation := drivers.FSIsolationChroot

	// Start a server and client
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}

		pluginConfig := []*nconfig.PluginConfig{
			{
				Name: "mock_driver",
				Config: map[string]interface{}{
					"fs_isolation": string(isolation),
				},
			},
		}

		c.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", map[string]string{}, pluginConfig)
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyAllocExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec})
	tokenAllocExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "alloc-exec", policyAllocExec)

	policyAllocNodeExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec, acl.NamespaceCapabilityAllocNodeExec})
	tokenAllocNodeExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "alloc-node-exec", policyAllocNodeExec)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"stdout_string": "some output",
		},
	}

	// Wait for client to be running job
	testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)

	// Get the allocation ID
	args := nstructs.AllocListRequest{}
	args.Region = "global"
	args.AuthToken = root.SecretID
	args.Namespace = nstructs.DefaultNamespace
	resp := nstructs.AllocListResponse{}
	require.NoError(t, s.RPC("Alloc.List", &args, &resp))
	require.Len(t, resp.Allocations, 1)
	allocID := resp.Allocations[0].ID

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "alloc-exec token",
			Token:         tokenAllocExec.SecretID,
			ExpectedError: "",
		},
		{
			Name:          "alloc-node-exec token",
			Token:         tokenAllocNodeExec.SecretID,
			ExpectedError: "",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request
			req := &cstructs.AllocExecRequest{
				AllocID: allocID,
				Task:    job.TaskGroups[0].Tasks[0].Name,
				Tty:     true,
				Cmd:     []string{"placeholder command"},
				QueryOptions: nstructs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("Allocations.Exec")
			require.Nil(t, err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

			// Start the handler
			go handler(p2)
			go decodeFrames(t, p1, frames, errCh)

			// Send the request
			encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
			require.Nil(t, encoder.Encode(req))

			select {
			case <-time.After(3 * time.Second):
			case err := <-errCh:
				if c.ExpectedError == "" {
					require.NoError(t, err)
				} else {
					require.Contains(t, err.Error(), c.ExpectedError)
				}
			case f := <-frames:
				// we are good if we don't expect an error
				if c.ExpectedError != "" {
					require.Fail(t, "unexpected frame", "frame: %#v", f)
				}
			}
		})
	}
}

// TestAlloc_ExecStreaming_ACL_WithIsolation_None asserts that token needs
// alloc-node-exec acl policy as well when no isolation is used
func TestAlloc_ExecStreaming_ACL_WithIsolation_None(t *testing.T) {
	ci.Parallel(t)
	isolation := drivers.FSIsolationNone

	// Start a server and client
	s, root, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}

		pluginConfig := []*nconfig.PluginConfig{
			{
				Name: "mock_driver",
				Config: map[string]interface{}{
					"fs_isolation": string(isolation),
				},
			},
		}

		c.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", map[string]string{}, pluginConfig)
	})
	defer cleanup()

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityDeny})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyAllocExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec})
	tokenAllocExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "alloc-exec", policyAllocExec)

	policyAllocNodeExec := mock.NamespacePolicy(nstructs.DefaultNamespace, "",
		[]string{acl.NamespaceCapabilityAllocExec, acl.NamespaceCapabilityAllocNodeExec})
	tokenAllocNodeExec := mock.CreatePolicyAndToken(t, s.State(), 1009, "alloc-node-exec", policyAllocNodeExec)

	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"stdout_string": "some output",
		},
	}

	// Wait for client to be running job
	testutil.WaitForRunningWithToken(t, s.RPC, job, root.SecretID)

	// Get the allocation ID
	args := nstructs.AllocListRequest{}
	args.Region = "global"
	args.AuthToken = root.SecretID
	args.Namespace = nstructs.DefaultNamespace
	resp := nstructs.AllocListResponse{}
	require.NoError(t, s.RPC("Alloc.List", &args, &resp))
	require.Len(t, resp.Allocations, 1)
	allocID := resp.Allocations[0].ID

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "alloc-exec token",
			Token:         tokenAllocExec.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "alloc-node-exec token",
			Token:         tokenAllocNodeExec.SecretID,
			ExpectedError: "",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request
			req := &cstructs.AllocExecRequest{
				AllocID: allocID,
				Task:    job.TaskGroups[0].Tasks[0].Name,
				Tty:     true,
				Cmd:     []string{"placeholder command"},
				QueryOptions: nstructs.QueryOptions{
					Region:    "global",
					AuthToken: c.Token,
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Get the handler
			handler, err := client.StreamingRpcHandler("Allocations.Exec")
			require.Nil(t, err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

			// Start the handler
			go handler(p2)
			go decodeFrames(t, p1, frames, errCh)

			// Send the request
			encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
			require.Nil(t, encoder.Encode(req))

			select {
			case <-time.After(3 * time.Second):
			case err := <-errCh:
				if c.ExpectedError == "" {
					require.NoError(t, err)
				} else {
					require.Contains(t, err.Error(), c.ExpectedError)
				}
			case f := <-frames:
				// we are good if we don't expect an error
				if c.ExpectedError != "" {
					require.Fail(t, "unexpected frame", "frame: %#v", f)
				}
			}
		})
	}
}

func decodeFrames(t *testing.T, p1 net.Conn, frames chan<- *drivers.ExecTaskStreamingResponseMsg, errCh chan<- error) {
	// Start the decoder
	decoder := codec.NewDecoder(p1, nstructs.MsgpackHandle)

	for {
		var msg cstructs.StreamErrWrapper
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				return
			}
			t.Logf("received error decoding: %#v", err)

			errCh <- fmt.Errorf("error decoding: %v", err)
			return
		}

		if msg.Error != nil {
			errCh <- msg.Error
			continue
		}

		var frame drivers.ExecTaskStreamingResponseMsg
		if err := json.Unmarshal(msg.Payload, &frame); err != nil {
			errCh <- err
			return
		}
		t.Logf("received message: %#v", msg)
		frames <- &frame
	}
}
