// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// TestClient_SelfDrainConfig is an integration test of the client's Leave
// method that exercises the behavior of the drain_on_shutdown configuration
func TestClient_SelfDrainConfig(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanupSRV := testServer(t, nil)
	defer cleanupSRV()
	testutil.WaitForLeader(t, srv.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = srv
		c.DevMode = false
		c.Drain = &config.DrainConfig{
			Deadline:         10 * time.Second,
			IgnoreSystemJobs: true,
		}
	})
	defer cleanupC1()

	jobID := "service-job-" + uuid.Short()
	sysJobID := "system-job-" + uuid.Short()
	testSelfDrainSetup(t, srv, c1.Node().ID, jobID, sysJobID)
	t.Log("setup complete successful, self-draining node")

	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- c1.Leave()
	}()

	select {
	case err := <-errCh:
		must.NoError(t, err)
	case <-testCtx.Done():
		t.Fatal("expected drain complete before deadline")
	}

	c1.allocLock.RLock()
	defer c1.allocLock.RUnlock()
	for _, runner := range c1.allocs {
		if runner.Alloc().JobID == sysJobID {
			must.Eq(t, structs.AllocClientStatusRunning, runner.AllocState().ClientStatus)
		} else {
			must.Eq(t, structs.AllocClientStatusComplete, runner.AllocState().ClientStatus)
		}
	}

}

// TestClient_SelfDrain_FailLocal is an integration test of the client's Leave
// method that exercises the behavior when the client loses connection with the
// server
func TestClient_SelfDrain_FailLocal(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanupSRV := testServer(t, nil)
	defer cleanupSRV()
	testutil.WaitForLeader(t, srv.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = srv
		c.DevMode = false
		c.Drain = &config.DrainConfig{Deadline: 5 * time.Second}
	})
	defer cleanupC1()

	jobID := "service-job-" + uuid.Short()
	sysJobID := "system-job-" + uuid.Short()
	testSelfDrainSetup(t, srv, c1.Node().ID, jobID, sysJobID)

	t.Log("setup complete successful, self-draining node and disconnecting node from server")

	// note: this timeout has to cover the drain deadline plus the RPC timeout
	// when we fail to make the RPC to the leader
	testCtx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- c1.Leave()
	}()

	// We want to disconnect the server so that self-drain is forced to fallback
	// to local drain behavior. But if we disconnect the server before we start
	// the self-drain, the drain won't happen at all. So this attempts to
	// interleave disconnecting the server between when the drain starts and the
	// server marks the drain successful.
	go func() {
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		var out structs.SingleNodeResponse
		for {
			select {
			case <-testCtx.Done():
				return
			default:
			}
			err := srv.RPC("Node.GetNode", &req, &out)
			must.NoError(t, err)
			if out.Node.DrainStrategy != nil {
				cleanupSRV()
				return
			} else if out.Node.LastDrain != nil {
				return // the drain is already complete
			}
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			// we might not be able to interleave the disconnection, so it's
			// possible the Leave works just fine
			must.EqError(t, err, "self-drain exceeded deadline")
		}
	case <-testCtx.Done():
		t.Fatal("expected drain complete before test timeout")
	}
}

func testSelfDrainSetup(t *testing.T, srv *nomad.Server, nodeID, jobID, sysJobID string) {
	req := structs.NodeSpecificRequest{
		NodeID:       nodeID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse

	// Wait for the node to register before we drain
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			err := srv.RPC("Node.GetNode", &req, &out)
			must.NoError(t, err)
			return out.Node != nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(10*time.Millisecond),
	))

	// Run a job that starts quickly
	job := mock.Job()
	job.ID = jobID
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Migrate = nstructs.DefaultMigrateStrategy()
	job.TaskGroups[0].Migrate.MinHealthyTime = 100 * time.Millisecond
	job.TaskGroups[0].Networks = []*structs.NetworkResource{}
	job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:      "mock",
		Driver:    "mock_driver",
		Config:    map[string]interface{}{"run_for": "1m"},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      50,
			MemoryMB: 25,
		},
	}
	testutil.WaitForRunning(t, srv.RPC, job.Copy())

	sysJob := mock.SystemJob()
	sysJob.ID = sysJobID
	sysJob.Constraints = nil
	sysJob.TaskGroups[0].Constraints = nil
	sysJob.TaskGroups[0].Networks = []*structs.NetworkResource{}
	sysJob.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:      "mock",
		Driver:    "mock_driver",
		Config:    map[string]interface{}{"run_for": "1m"},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      50,
			MemoryMB: 25,
		},
	}
	testutil.WaitForRunning(t, srv.RPC, sysJob.Copy())

}
