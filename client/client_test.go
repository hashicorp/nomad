// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/device"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
)

func testACLServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string, *structs.ACLToken, func()) {
	server, token, cleanup := nomad.TestACLServer(t, cb)
	return server, server.GetConfig().RPCAddr.String(), token, cleanup
}

func testServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string, func()) {
	server, cleanup := nomad.TestServer(t, cb)
	return server, server.GetConfig().RPCAddr.String(), cleanup
}

func TestClient_StartStop(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Certain labels for metrics are dependant on client initial setup. This tests
// that the client has properly initialized before we assign values to labels
func TestClient_BaseLabels(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	client, cleanup := TestClient(t, nil)
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer cleanup()

	// directly invoke this function, as otherwise this will fail on a CI build
	// due to a race condition
	client.emitStats()

	baseLabels := client.baseLabels
	assert.NotEqual(0, len(baseLabels))

	nodeID := client.Node().ID
	for _, e := range baseLabels {
		if e.Name == "node_id" {
			assert.Equal(nodeID, e.Value)
		}
	}
}

func TestClient_RPC(t *testing.T) {
	ci.Parallel(t)

	_, addr, cleanupS1 := testServer(t, nil)
	defer cleanupS1()

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanupC1()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", &structs.GenericRequest{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_RPC_FireRetryWatchers(t *testing.T) {
	ci.Parallel(t)

	_, addr, cleanupS1 := testServer(t, nil)
	defer cleanupS1()

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanupC1()

	watcher := c1.rpcRetryWatcher()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", &structs.GenericRequest{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	select {
	case <-watcher:
	default:
		t.Fatal("watcher should be fired")
	}
}

func TestClient_RPC_Passthrough(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanupC1()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", &structs.GenericRequest{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Ensure we are fingerprinting
	testutil.WaitForResult(func() (bool, error) {
		node := c.Node()
		if _, ok := node.Attributes["kernel.name"]; !ok {
			return false, fmt.Errorf("Expected value for kernel.name")
		}
		if _, ok := node.Attributes["cpu.arch"]; !ok {
			return false, fmt.Errorf("Expected value for cpu.arch")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestClient_Fingerprint_Periodic asserts that driver node attributes are
// periodically fingerprinted.
func TestClient_Fingerprint_Periodic(t *testing.T) {
	ci.Parallel(t)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		confs := []*nconfig.PluginConfig{
			{
				Name: "mock_driver",
				Config: map[string]interface{}{
					"shutdown_periodic_after":    true,
					"shutdown_periodic_duration": time.Second,
				},
			},
		}
		c.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", nil, confs)
	})
	defer cleanup()

	// Ensure the mock driver is registered on the client
	testutil.WaitForResult(func() (bool, error) {
		node := c1.Node()

		// assert that the driver is set on the node attributes
		mockDriverInfoAttr := node.Attributes["driver.mock_driver"]
		if mockDriverInfoAttr == "" {
			return false, fmt.Errorf("mock driver is empty when it should be set on the node attributes")
		}

		mockDriverInfo := node.Drivers["mock_driver"]

		// assert that the Driver information for the node is also set correctly
		if mockDriverInfo == nil {
			return false, fmt.Errorf("mock driver is nil when it should be set on node Drivers")
		}
		if !mockDriverInfo.Detected {
			return false, fmt.Errorf("mock driver should be set as detected")
		}
		if !mockDriverInfo.Healthy {
			return false, fmt.Errorf("mock driver should be set as healthy")
		}
		if mockDriverInfo.HealthDescription == "" {
			return false, fmt.Errorf("mock driver description should not be empty")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	testutil.WaitForResult(func() (bool, error) {
		mockDriverInfo := c1.Node().Drivers["mock_driver"]
		// assert that the Driver information for the node is also set correctly
		if mockDriverInfo == nil {
			return false, fmt.Errorf("mock driver is nil when it should be set on node Drivers")
		}
		if mockDriverInfo.Detected {
			return false, fmt.Errorf("mock driver should not be set as detected")
		}
		if mockDriverInfo.Healthy {
			return false, fmt.Errorf("mock driver should not be set as healthy")
		}
		if mockDriverInfo.HealthDescription == "" {
			return false, fmt.Errorf("mock driver description should not be empty")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestClient_MixedTLS asserts that when a server is running with TLS enabled
// it will reject any RPC connections from clients that lack TLS. See #2525
func TestClient_MixedTLS(t *testing.T) {
	ci.Parallel(t)

	const (
		cafile        = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooservercert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		fooserverkey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
	)
	s1, addr, cleanupS1 := testServer(t, func(c *nomad.Config) {
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooservercert,
			KeyFile:              fooserverkey,
		}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	// tell the client we've registered to unblock the RPC we test below
	c1.registeredOnce.Do(func() { close(c1.registeredCh) })

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse
	testutil.AssertUntil(100*time.Millisecond,
		func() (bool, error) {
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil || structs.IsErrPermissionDenied(err) {
				return false, fmt.Errorf("client RPC succeeded when it should have failed:\n%+v", out)
			}
			return true, nil
		},
		func(err error) {
			t.Fatalf(err.Error())
		},
	)
}

// TestClient_BadTLS asserts that when a client and server are running with TLS
// enabled -- but their certificates are signed by different CAs -- they're
// unable to communicate.
func TestClient_BadTLS(t *testing.T) {
	ci.Parallel(t)

	const (
		cafile        = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooclientcert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fooclientkey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
		badca         = "../helper/tlsutil/testdata/bad-agent-ca.pem"
		badcert       = "../helper/tlsutil/testdata/badRegion-client-bad.pem"
		badkey        = "../helper/tlsutil/testdata/badRegion-client-bad-key.pem"
	)
	s1, addr, cleanupS1 := testServer(t, func(c *nomad.Config) {
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooclientcert,
			KeyFile:              fooclientkey,
		}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               badca,
			CertFile:             badcert,
			KeyFile:              badkey,
		}
	})
	defer cleanupC1()

	// tell the client we've registered to unblock the RPC we test below
	c1.registeredOnce.Do(func() { close(c1.registeredCh) })

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse
	testutil.AssertUntil(100*time.Millisecond,
		func() (bool, error) {
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil || structs.IsErrPermissionDenied(err) {
				return false, fmt.Errorf("client RPC succeeded when it should have failed:\n%+v", out)
			}
			return true, nil
		},
		func(err error) {
			t.Fatalf(err.Error())
		},
	)
}

func TestClient_Register(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanupC1()

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse

	// Register should succeed
	testutil.WaitForResult(func() (bool, error) {
		err := s1.RPC("Node.GetNode", &req, &out)
		if err != nil {
			return false, err
		}
		if out.Node == nil {
			return false, fmt.Errorf("missing reg")
		}
		return out.Node.ID == req.NodeID, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_Register_NodePool(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create client with a node pool configured.
	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Node.NodePool = "dev"
	})
	defer cleanupC1()

	// Create client with no node pool configured.
	c2, cleanupC2 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.Node.NodePool = ""
	})
	defer cleanupC2()

	nodeReq := structs.NodeSpecificRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var nodeResp structs.SingleNodeResponse

	poolReq := structs.NodePoolSpecificRequest{
		Name:         "dev",
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var poolResp structs.SingleNodePoolResponse

	// Register should succeed and node pool should be created.
	// Client without node pool configuration should be in the default pool.
	testutil.WaitForResult(func() (bool, error) {
		// Fetch node1.
		nodeReq.NodeID = c1.Node().ID
		err := s1.RPC("Node.GetNode", &nodeReq, &nodeResp)
		if err != nil {
			return false, err
		}
		if nodeResp.Node == nil {
			return false, fmt.Errorf("c1 is missing")
		}
		if nodeResp.Node.NodePool != "dev" {
			return false, fmt.Errorf("c1 has wrong node pool")
		}

		// Fetch node1 node pool.
		err = s1.RPC("NodePool.GetNodePool", &poolReq, &poolResp)
		if err != nil {
			return false, err
		}
		if poolResp.NodePool == nil {
			return false, fmt.Errorf("dev node pool is nil")
		}

		// Fetch node2.
		nodeReq.NodeID = c2.Node().ID
		err = s1.RPC("Node.GetNode", &nodeReq, &nodeResp)
		if err != nil {
			return false, err
		}
		if nodeResp.Node == nil {
			return false, fmt.Errorf("c2 is missing")
		}
		if nodeResp.Node.NodePool != structs.NodePoolDefault {
			return false, fmt.Errorf("c2 has wrong node pool")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_Heartbeat(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, func(c *nomad.Config) {
		c.MinHeartbeatTTL = 50 * time.Millisecond
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanupC1()

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse

	// Register should succeed
	testutil.WaitForResult(func() (bool, error) {
		err := s1.RPC("Node.GetNode", &req, &out)
		if err != nil {
			return false, err
		}
		if out.Node == nil {
			return false, fmt.Errorf("missing reg")
		}
		return out.Node.Status == structs.NodeStatusReady, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestClient_UpdateAllocStatus that once running allocations send updates to
// the server.
func TestClient_UpdateAllocStatus(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()

	_, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	job := mock.Job()
	// allow running job on any node including self client, that may not be a Linux box
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = 1
	task := job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	task.Services = nil

	// WaitForRunning polls the server until the ClientStatus is running
	testutil.WaitForRunning(t, s1.RPC, job)
}

func TestClient_WatchAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	// Create mock allocations
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}
	alloc1 := mock.Alloc()
	alloc1.JobID = job.ID
	alloc1.Job = job
	alloc1.NodeID = c1.Node().ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = c1.Node().ID
	alloc2.JobID = job.ID
	alloc2.Job = job

	state := s1.State()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job); err != nil {
		t.Fatal(err)
	}
	if err := state.UpsertJobSummary(101, mock.JobSummary(alloc1.JobID)); err != nil {
		t.Fatal(err)
	}
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Both allocations should get registered
	testutil.WaitForResult(func() (bool, error) {
		c1.allocLock.RLock()
		num := len(c1.allocs)
		c1.allocLock.RUnlock()
		return num == 2, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Delete one allocation
	if err := state.DeleteEval(103, nil, []string{alloc1.ID}, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the other allocation. Have to make a copy because the allocs are
	// shared in memory in the test and the modify index would be updated in the
	// alloc runner.
	alloc2_2 := alloc2.Copy()
	alloc2_2.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 104, []*structs.Allocation{alloc2_2}); err != nil {
		t.Fatalf("err upserting stopped alloc: %v", err)
	}

	// One allocation should get GC'd and removed
	testutil.WaitForResult(func() (bool, error) {
		c1.allocLock.RLock()
		num := len(c1.allocs)
		c1.allocLock.RUnlock()
		return num == 1, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// One allocations should get updated
	testutil.WaitForResult(func() (bool, error) {
		c1.allocLock.RLock()
		ar := c1.allocs[alloc2.ID]
		c1.allocLock.RUnlock()
		return ar.Alloc().DesiredStatus == structs.AllocDesiredStatusStop, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func waitTilNodeReady(client *Client, t *testing.T) {
	testutil.WaitForResult(func() (bool, error) {
		n := client.Node()
		if n.Status != structs.NodeStatusReady {
			return false, fmt.Errorf("node not registered")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestClient_SaveRestoreState exercises the allocrunner restore code paths
// after a client restart. It runs several jobs in different states and asserts
// the expected final state and server updates.
func TestClient_SaveRestoreState(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	t.Cleanup(cleanupS1)
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	t.Cleanup(func() {
		for _, ar := range c1.getAllocRunners() {
			ar.Destroy()
		}
		for _, ar := range c1.getAllocRunners() {
			<-ar.DestroyCh()
		}
		cleanupC1()
	})

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	migrateStrategy := structs.DefaultMigrateStrategy()
	migrateStrategy.MinHealthyTime = time.Millisecond
	migrateStrategy.HealthCheck = structs.MigrateStrategyHealthStates

	// Create mock jobs and allocations that will start up fast

	setup := func(id string) *structs.Job {
		job := mock.MinJob()
		job.ID = id
		job.TaskGroups[0].Migrate = migrateStrategy
		must.NoError(t, s1.RPC("Job.Register", &structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global", Namespace: job.Namespace},
		}, &structs.JobRegisterResponse{}))
		return job
	}

	// job1: will be left running
	// job2: will be stopped before shutdown
	// job3: will be stopped after shutdown
	// job4: will be stopped and GC'd after shutdown
	job1, job2, job3, job4 := setup("job1"), setup("job2"), setup("job3"), setup("job4")

	// Allocations should be placed
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			c1.allocLock.RLock()
			defer c1.allocLock.RUnlock()
			if len(c1.allocs) != 4 {
				return fmt.Errorf("expected 4 alloc runners")
			}
			for _, ar := range c1.allocs {
				if ar.AllocState().ClientStatus != structs.AllocClientStatusRunning {
					return fmt.Errorf("expected running client status, got %v",
						ar.AllocState().ClientStatus)
				}
			}
			return nil
		}),
		wait.Timeout(time.Second*10),
		wait.Gap(time.Millisecond*30),
	))

	store := s1.State()

	allocIDforJob := func(job *structs.Job) string {
		allocs, err := store.AllocsByJob(nil, job.Namespace, job.ID, false)
		must.NoError(t, err)
		must.Len(t, 1, allocs) // we should only ever get 1 in this test
		return allocs[0].ID
	}
	alloc1 := allocIDforJob(job1)
	alloc2 := allocIDforJob(job2)
	alloc3 := allocIDforJob(job3)
	alloc4 := allocIDforJob(job4)
	t.Logf("alloc1=%s alloc2=%s alloc3=%s alloc4=%s", alloc1, alloc2, alloc3, alloc4)

	// Stop the 2nd job before we shut down
	must.NoError(t, s1.RPC("Job.Deregister", &structs.JobDeregisterRequest{
		JobID:        job2.ID,
		WriteRequest: structs.WriteRequest{Region: "global", Namespace: job2.Namespace},
	}, &structs.JobDeregisterResponse{}))

	var alloc2ModifyIndex uint64
	var alloc2AllocModifyIndex uint64

	// Wait till we're sure the client has received the stop and updated the server
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			alloc, err := store.AllocByID(nil, alloc2)
			must.NotNil(t, alloc)
			must.NoError(t, err)
			if alloc.ClientStatus != structs.AllocClientStatusComplete {
				// note that the allocrunner is non-nil until it's been
				// client-GC'd, so we're just looking to make sure the client
				// has updated the server
				return fmt.Errorf("alloc2 should have been marked completed")
			}
			alloc2ModifyIndex = alloc.ModifyIndex
			alloc2AllocModifyIndex = alloc.AllocModifyIndex
			return nil
		}),
		wait.Timeout(time.Second*20),
		wait.Gap(time.Millisecond*30),
	))

	t.Log("shutting down client")
	must.NoError(t, c1.Shutdown()) // note: this saves the client state DB

	// Stop the 3rd job while we're down
	must.NoError(t, s1.RPC("Job.Deregister", &structs.JobDeregisterRequest{
		JobID:        job3.ID,
		WriteRequest: structs.WriteRequest{Region: "global", Namespace: job3.Namespace},
	}, &structs.JobDeregisterResponse{}))

	// Stop and purge the 4th job while we're down
	must.NoError(t, s1.RPC("Job.Deregister", &structs.JobDeregisterRequest{
		JobID:        job4.ID,
		Purge:        true,
		WriteRequest: structs.WriteRequest{Region: "global", Namespace: job4.Namespace},
	}, &structs.JobDeregisterResponse{}))

	// Ensure the allocation has been deleted as well
	must.NoError(t, s1.RPC("Eval.Reap", &structs.EvalReapRequest{
		Allocs:       []string{alloc4},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}, &structs.GenericResponse{}))

	var alloc3AllocModifyIndex uint64
	var alloc3ModifyIndex uint64

	// Wait till we're sure the scheduler has marked alloc3 for stop and deleted alloc4
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			alloc, err := store.AllocByID(nil, alloc3)
			must.NotNil(t, alloc)
			must.NoError(t, err)
			if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
				return fmt.Errorf("alloc3 should have been marked for stop")
			}
			alloc3ModifyIndex = alloc.ModifyIndex
			alloc3AllocModifyIndex = alloc.AllocModifyIndex

			alloc, err = store.AllocByID(nil, alloc4)
			must.NoError(t, err)
			if alloc != nil {
				return fmt.Errorf("alloc4 should have been deleted")
			}
			return nil
		}),
		wait.Timeout(time.Second*5),
		wait.Gap(time.Millisecond*30),
	))

	a1, err := store.AllocByID(nil, alloc1)
	var alloc1AllocModifyIndex uint64
	var alloc1ModifyIndex uint64
	alloc1ModifyIndex = a1.ModifyIndex
	alloc1AllocModifyIndex = a1.AllocModifyIndex

	t.Log("starting new client")

	logger := testlog.HCLogger(t)
	c1.config.Logger = logger
	consulCatalog := consul.NewMockCatalog(logger)
	mockService := regMock.NewServiceRegistrationHandler(logger)

	// ensure we use non-shutdown driver instances
	c1.config.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", c1.config.Options, nil)
	c1.config.PluginSingletonLoader = singleton.NewSingletonLoader(logger, c1.config.PluginLoader)

	c2, err := NewClient(c1.config, consulCatalog, nil, mockService, nil)
	must.NoError(t, err)

	t.Cleanup(func() {
		for _, ar := range c2.getAllocRunners() {
			ar.Destroy()
		}
		for _, ar := range c2.getAllocRunners() {
			<-ar.DestroyCh()
		}
		c2.Shutdown()
	})

	// Ensure only the expected allocation is running
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			c2.allocLock.RLock()
			defer c2.allocLock.RUnlock()
			if len(c2.allocs) != 3 {
				// the GC'd alloc will not have restored AR
				return fmt.Errorf("expected 3 alloc runners")
			}
			for allocID, ar := range c2.allocs {
				if ar == nil {
					return fmt.Errorf("nil alloc runner")
				}
				switch allocID {
				case alloc1:
					if ar.AllocState().ClientStatus != structs.AllocClientStatusRunning {
						return fmt.Errorf("expected running client status, got %v",
							ar.AllocState().ClientStatus)
					}

				case alloc3:
					if ar.AllocState().ClientStatus != structs.AllocClientStatusComplete {
						return fmt.Errorf("expected complete client status, got %v",
							ar.AllocState().ClientStatus)
					}

					// because the client's update will be batched, we need to
					// ensure we wait for the server update too
					a3, err := store.AllocByID(nil, alloc3)
					must.NoError(t, err)
					must.NotNil(t, a3)
					if alloc3AllocModifyIndex != a3.AllocModifyIndex ||
						alloc3ModifyIndex >= a3.ModifyIndex {
						return fmt.Errorf(
							"alloc %s stopped during shutdown should have updated", a3.ID[:8])
					}

				default:
					if ar.AllocState().ClientStatus != structs.AllocClientStatusComplete {
						return fmt.Errorf("expected complete client status, got %v",
							ar.AllocState().ClientStatus)
					}
				}
			}
			return nil
		}),
		wait.Timeout(time.Second*10),
		wait.Gap(time.Millisecond*30),
	))

	// Because we're asserting that no changes have been made, we have to wait a
	// sufficient amount of time to verify that
	must.Wait(t, wait.ContinualSuccess(
		wait.ErrorFunc(func() error {
			a1, err = store.AllocByID(nil, alloc1)
			must.NoError(t, err)
			must.NotNil(t, a1)

			if alloc1AllocModifyIndex != a1.AllocModifyIndex ||
				alloc1ModifyIndex != a1.ModifyIndex {
				return fmt.Errorf("alloc still running should not have updated")
			}

			a2, err := store.AllocByID(nil, alloc2)
			must.NoError(t, err)
			must.NotNil(t, a2)
			if alloc2AllocModifyIndex != a2.AllocModifyIndex ||
				alloc2ModifyIndex != a2.ModifyIndex {
				return fmt.Errorf(
					"alloc %s stopped before shutdown should not have updated", a2.ID[:8])
			}

			// TODO: the alloc has been GC'd so the server will reject any
			// update. It'd be nice if we could instrument the server here to
			// ensure we didn't send one either.
			a4, err := store.AllocByID(nil, alloc4)
			must.NoError(t, err)
			if a4 != nil {
				return fmt.Errorf("garbage collected alloc should not exist")
			}

			return nil
		}),
		wait.Timeout(time.Second*3),
		wait.Gap(time.Millisecond*100),
	))

}

func TestClient_AddAllocError(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	defer cleanupC1()

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	// Create mock allocation with invalid task group name
	job := mock.Job()
	alloc1 := mock.Alloc()
	alloc1.NodeID = c1.Node().ID
	alloc1.Job = job
	alloc1.JobID = job.ID
	alloc1.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	alloc1.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}
	alloc1.ClientStatus = structs.AllocClientStatusPending

	// Set these two fields to nil to cause alloc runner creation to fail
	alloc1.AllocatedResources = nil
	alloc1.TaskResources = nil

	state := s1.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	require.Nil(err)

	err = state.UpsertJobSummary(101, mock.JobSummary(alloc1.JobID))
	require.Nil(err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc1})
	require.Nil(err)

	// Push this alloc update to the client
	allocUpdates := &allocUpdates{
		pulled: map[string]*structs.Allocation{
			alloc1.ID: alloc1,
		},
	}
	c1.runAllocs(allocUpdates)

	// Ensure the allocation has been marked as invalid and failed on the server
	testutil.WaitForResult(func() (bool, error) {
		c1.allocLock.RLock()
		ar := c1.allocs[alloc1.ID]
		_, isInvalid := c1.invalidAllocs[alloc1.ID]
		c1.allocLock.RUnlock()
		if ar != nil {
			return false, fmt.Errorf("expected nil alloc runner")
		}
		if !isInvalid {
			return false, fmt.Errorf("expected alloc to be marked as invalid")
		}
		alloc, err := s1.State().AllocByID(nil, alloc1.ID)
		require.Nil(err)
		failed := alloc.ClientStatus == structs.AllocClientStatusFailed
		if !failed {
			return false, fmt.Errorf("Expected failed client status, but got %v", alloc.ClientStatus)
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

}

func TestClient_Init(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

	allocDir := filepath.Join(dir, "alloc")

	config := config.DefaultConfig()
	config.AllocDir = allocDir
	config.StateDBFactory = cstate.GetStateDBFactory(true)

	// Node is always initialized in agent.go:convertClientConfig()
	config.Node = mock.Node()

	client := &Client{
		config: config,
		logger: testlog.HCLogger(t),
	}

	must.NoError(t, client.init())
	_, err := os.Stat(allocDir)
	must.NoError(t, err)
}

func TestClient_BlockedAllocations(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Wait for the node to be ready
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.NodeByID(ws, c1.Node().ID)
		if err != nil {
			return false, err
		}
		if out == nil || out.Status != structs.NodeStatusReady {
			return false, fmt.Errorf("bad node: %#v", out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Add an allocation
	alloc := mock.Alloc()
	alloc.NodeID = c1.Node().ID
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"kill_after":  "1s",
		"run_for":     "100s",
		"exit_code":   0,
		"exit_signal": 0,
	}

	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 100, []*structs.Allocation{alloc})

	// Wait until the client downloads and starts the allocation
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.AllocByID(ws, alloc.ID)
		if err != nil {
			return false, err
		}
		if out == nil || out.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("bad alloc: %#v", out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Add a new chained alloc
	alloc2 := alloc.Copy()
	alloc2.ID = uuid.Generate()
	alloc2.Job = alloc.Job
	alloc2.JobID = alloc.JobID
	alloc2.PreviousAllocation = alloc.ID
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that the chained allocation is being tracked as blocked
	testutil.WaitForResult(func() (bool, error) {
		ar := c1.getAllocRunners()[alloc2.ID]
		if ar == nil {
			return false, fmt.Errorf("alloc 2's alloc runner does not exist")
		}
		if !ar.IsWaiting() {
			return false, fmt.Errorf("alloc 2 is not blocked")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Change the desired state of the parent alloc to stop
	alloc1 := alloc.Copy()
	alloc1.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 300, []*structs.Allocation{alloc1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that there are no blocked allocations
	testutil.WaitForResult(func() (bool, error) {
		for id, ar := range c1.getAllocRunners() {
			if ar.IsWaiting() {
				return false, fmt.Errorf("%q still blocked", id)
			}
			if ar.IsMigrating() {
				return false, fmt.Errorf("%q still migrating", id)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Destroy all the allocations
	for _, ar := range c1.getAllocRunners() {
		ar.Destroy()
	}

	for _, ar := range c1.getAllocRunners() {
		<-ar.DestroyCh()
	}
}

func TestClient_ValidateMigrateToken_ValidToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	c, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
	})
	defer cleanup()

	alloc := mock.Alloc()
	validToken, err := structs.GenerateMigrateToken(alloc.ID, c.secretNodeID())
	assert.Nil(err)

	assert.Equal(c.ValidateMigrateToken(alloc.ID, validToken), true)
}

func TestClient_ValidateMigrateToken_InvalidToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	c, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
	})
	defer cleanup()

	assert.Equal(c.ValidateMigrateToken("", ""), false)

	alloc := mock.Alloc()
	assert.Equal(c.ValidateMigrateToken(alloc.ID, alloc.ID), false)
	assert.Equal(c.ValidateMigrateToken(alloc.ID, ""), false)
}

func TestClient_ValidateMigrateToken_ACLDisabled(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	c, cleanup := TestClient(t, func(c *config.Config) {})
	defer cleanup()

	assert.Equal(c.ValidateMigrateToken("", ""), true)
}

func TestClient_ReloadTLS_UpgradePlaintextToTLS(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	s1, addr, cleanupS1 := testServer(t, func(c *nomad.Config) {
		c.Region = "global"
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	const (
		cafile        = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooclientcert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fooclientkey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	// Registering a node over plaintext should succeed
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}

		testutil.WaitForResult(func() (bool, error) {
			var out structs.SingleNodeResponse
			err := c1.RPC("Node.GetNode", &req, &out)
			if err != nil {
				return false, fmt.Errorf("client RPC failed when it should have succeeded:\n%+v", err)
			}
			return true, nil
		},
			func(err error) {
				t.Fatalf(err.Error())
			},
		)
	}

	newConfig := &nconfig.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               cafile,
		CertFile:             fooclientcert,
		KeyFile:              fooclientkey,
	}

	err := c1.reloadTLSConnections(newConfig)
	assert.Nil(err)

	// Registering a node over plaintext should fail after the node has upgraded
	// to TLS
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		testutil.WaitForResult(func() (bool, error) {
			var out structs.SingleNodeResponse
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil {
				return false, fmt.Errorf("client RPC succeeded when it should have failed:\n%+v", err)
			}
			return true, nil
		},
			func(err error) {
				t.Fatalf(err.Error())
			},
		)
	}
}

func TestClient_ReloadTLS_DowngradeTLSToPlaintext(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	s1, addr, cleanupS1 := testServer(t, func(c *nomad.Config) {
		c.Region = "global"
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	const (
		cafile        = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooclientcert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fooclientkey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooclientcert,
			KeyFile:              fooclientkey,
		}
	})
	defer cleanup()

	// tell the client we've registered to unblock the RPC we test below
	c1.registeredOnce.Do(func() { close(c1.registeredCh) })

	// assert that when one node is running in encrypted mode, a RPC request to a
	// node running in plaintext mode should fail
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		testutil.WaitForResult(func() (bool, error) {
			var out structs.SingleNodeResponse
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil || structs.IsErrPermissionDenied(err) {
				return false, fmt.Errorf("client RPC succeeded when it should have failed :\n%+v", err)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf(err.Error())
		},
		)
	}

	newConfig := &nconfig.TLSConfig{}

	err := c1.reloadTLSConnections(newConfig)
	assert.Nil(err)

	// assert that when both nodes are in plaintext mode, a RPC request should
	// succeed
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		testutil.WaitForResult(func() (bool, error) {
			var out structs.SingleNodeResponse
			err := c1.RPC("Node.GetNode", &req, &out)
			if err != nil {
				return false, fmt.Errorf("client RPC failed when it should have succeeded:\n%+v", err)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf(err.Error())
		},
		)
	}
}

// TestClient_ServerList tests client methods that interact with the internal
// nomad server list.
func TestClient_ServerList(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, func(c *config.Config) {})
	defer cleanup()

	if s := client.GetServers(); len(s) != 0 {
		t.Fatalf("expected server lit to be empty but found: %+q", s)
	}
	if _, err := client.SetServers(nil); err != noServersErr {
		t.Fatalf("expected setting an empty list to return a 'no servers' error but received %v", err)
	}
	if _, err := client.SetServers([]string{"123.456.13123.123.13:80"}); err == nil {
		t.Fatalf("expected setting a bad server to return an error")
	}
	if _, err := client.SetServers([]string{"123.456.13123.123.13:80", "127.0.0.1:1234", "127.0.0.1"}); err == nil {
		t.Fatalf("expected setting at least one good server to succeed but received: %v", err)
	}
	s := client.GetServers()
	if len(s) != 0 {
		t.Fatalf("expected 2 servers but received: %+q", s)
	}
}

func TestClient_UpdateNodeFromDevicesAccumulates(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, func(c *config.Config) {})
	defer cleanup()

	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{CpuShares: 123},
		},
	})

	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Memory: structs.NodeMemoryResources{MemoryMB: 1024},
		},
	})

	client.updateNodeFromDevices([]*structs.NodeDeviceResource{
		{
			Vendor: "vendor",
			Type:   "type",
		},
	})

	// initial check
	conf := client.GetConfig()
	expectedResources := &structs.NodeResources{
		// computed through test client initialization
		Networks:     conf.Node.NodeResources.Networks,
		NodeNetworks: conf.Node.NodeResources.NodeNetworks,
		Disk:         conf.Node.NodeResources.Disk,

		// injected
		Cpu: structs.NodeCpuResources{
			CpuShares:          123,
			ReservableCpuCores: conf.Node.NodeResources.Cpu.ReservableCpuCores,
			TotalCpuCores:      conf.Node.NodeResources.Cpu.TotalCpuCores,
		},
		Memory: structs.NodeMemoryResources{MemoryMB: 1024},
		Devices: []*structs.NodeDeviceResource{
			{
				Vendor: "vendor",
				Type:   "type",
			},
		},
	}

	assert.EqualValues(t, expectedResources, conf.Node.NodeResources)

	// overrides of values

	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Memory: structs.NodeMemoryResources{MemoryMB: 2048},
		},
	})

	client.updateNodeFromDevices([]*structs.NodeDeviceResource{
		{
			Vendor: "vendor",
			Type:   "type",
		},
		{
			Vendor: "vendor2",
			Type:   "type2",
		},
	})

	conf = client.GetConfig()

	expectedResources2 := &structs.NodeResources{
		// computed through test client initialization
		Networks:     conf.Node.NodeResources.Networks,
		NodeNetworks: conf.Node.NodeResources.NodeNetworks,
		Disk:         conf.Node.NodeResources.Disk,

		// injected
		Cpu: structs.NodeCpuResources{
			CpuShares:          123,
			ReservableCpuCores: conf.Node.NodeResources.Cpu.ReservableCpuCores,
			TotalCpuCores:      conf.Node.NodeResources.Cpu.TotalCpuCores,
		},
		Memory: structs.NodeMemoryResources{MemoryMB: 2048},
		Devices: []*structs.NodeDeviceResource{
			{
				Vendor: "vendor",
				Type:   "type",
			},
			{
				Vendor: "vendor2",
				Type:   "type2",
			},
		},
	}

	assert.EqualValues(t, expectedResources2, conf.Node.NodeResources)

}

// TestClient_UpdateNodeFromFingerprintKeepsConfig asserts manually configured
// network interfaces take precedence over fingerprinted ones.
func TestClient_UpdateNodeFromFingerprintKeepsConfig(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS != "linux" {
		t.Skip("assertions assume linux platform")
	}

	// Client without network configured updates to match fingerprint
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu:      structs.NodeCpuResources{CpuShares: 123},
			Networks: []*structs.NetworkResource{{Mode: "host", Device: "any-interface"}},
		},
		Resources: &structs.Resources{
			CPU: 80,
		},
	})
	idx := len(client.config.Node.NodeResources.Networks) - 1
	require.Equal(t, int64(123), client.config.Node.NodeResources.Cpu.CpuShares)
	require.Equal(t, "any-interface", client.config.Node.NodeResources.Networks[idx].Device)
	require.Equal(t, 80, client.config.Node.Resources.CPU)

	// lookup an interface. client.Node starts with a hardcoded value, eth0,
	// and is only updated async through fingerprinter.
	// Let's just lookup network device; anyone will do for this test
	interfaces, err := net.Interfaces()
	require.NoError(t, err)
	require.NotEmpty(t, interfaces)
	dev := interfaces[0].Name

	// Client with network interface configured keeps the config
	// setting on update
	name := "TestClient_UpdateNodeFromFingerprintKeepsConfig2"
	client, cleanup = TestClient(t, func(c *config.Config) {
		c.NetworkInterface = dev
		c.Node.Name = name
		c.Options["fingerprint.denylist"] = "network"
		// Node is already a mock.Node, with a device
		c.Node.NodeResources.Networks[0].Device = dev
	})
	defer cleanup()
	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{CpuShares: 123},
			Networks: []*structs.NetworkResource{
				{Mode: "host", Device: "any-interface", MBits: 20},
			},
		},
	})
	require.Equal(t, int64(123), client.config.Node.NodeResources.Cpu.CpuShares)
	// only the configured device is kept
	require.Equal(t, 2, len(client.config.Node.NodeResources.Networks))
	require.Equal(t, dev, client.config.Node.NodeResources.Networks[0].Device)
	require.Equal(t, "bridge", client.config.Node.NodeResources.Networks[1].Mode)

	// Network speed is applied to all NetworkResources
	client.config.NetworkInterface = ""
	client.config.NetworkSpeed = 100
	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{CpuShares: 123},
			Networks: []*structs.NetworkResource{
				{Mode: "host", Device: "any-interface", MBits: 20},
			},
		},
		Resources: &structs.Resources{
			CPU: 80,
		},
	})
	assert.Equal(t, 3, len(client.config.Node.NodeResources.Networks))
	assert.Equal(t, "any-interface", client.config.Node.NodeResources.Networks[2].Device)
	assert.Equal(t, 100, client.config.Node.NodeResources.Networks[2].MBits)
	assert.Equal(t, 0, client.config.Node.NodeResources.Networks[1].MBits)
}

// Support multiple IP addresses (ipv4 vs. 6, e.g.) on the configured network interface
func Test_UpdateNodeFromFingerprintMultiIP(t *testing.T) {
	ci.Parallel(t)

	var dev string
	switch runtime.GOOS {
	case "linux":
		dev = "lo"
	case "darwin":
		dev = "lo0"
	}

	// Client without network configured updates to match fingerprint
	client, cleanup := TestClient(t, func(c *config.Config) {
		c.NetworkInterface = dev
		c.Options["fingerprint.denylist"] = "network,cni,bridge"
		c.Node.Resources.Networks = c.Node.NodeResources.Networks
	})
	defer cleanup()

	client.updateNodeFromFingerprint(&fingerprint.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{CpuShares: 123},
			Networks: []*structs.NetworkResource{
				{Device: dev, IP: "127.0.0.1"},
				{Device: dev, IP: "::1"},
			},
		},
	})

	nets := structs.Networks{
		mock.Node().NodeResources.Networks[0],
		{Device: dev, IP: "127.0.0.1"},
		{Device: dev, IP: "::1"},
	}

	require.Equal(t, nets, client.config.Node.NodeResources.Networks)
}

func TestClient_computeAllocatedDeviceStats(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	c := &Client{logger: logger}

	newDeviceStats := func(strValue string) *device.DeviceStats {
		return &device.DeviceStats{
			Summary: &psstructs.StatValue{
				StringVal: &strValue,
			},
		}
	}

	allocatedDevices := []*structs.AllocatedDeviceResource{
		{
			Vendor:    "vendor",
			Type:      "type",
			Name:      "name",
			DeviceIDs: []string{"d2", "d3", "notfoundid"},
		},
		{
			Vendor:    "vendor2",
			Type:      "type2",
			Name:      "name2",
			DeviceIDs: []string{"a2"},
		},
		{
			Vendor:    "vendor_notfound",
			Type:      "type_notfound",
			Name:      "name_notfound",
			DeviceIDs: []string{"d3"},
		},
	}

	hostDeviceGroupStats := []*device.DeviceGroupStats{
		{
			Vendor: "vendor",
			Type:   "type",
			Name:   "name",
			InstanceStats: map[string]*device.DeviceStats{
				"unallocated": newDeviceStats("unallocated"),
				"d2":          newDeviceStats("d2"),
				"d3":          newDeviceStats("d3"),
			},
		},
		{
			Vendor: "vendor2",
			Type:   "type2",
			Name:   "name2",
			InstanceStats: map[string]*device.DeviceStats{
				"a2": newDeviceStats("a2"),
			},
		},
		{
			Vendor: "vendor_unused",
			Type:   "type_unused",
			Name:   "name_unused",
			InstanceStats: map[string]*device.DeviceStats{
				"unallocated_unused": newDeviceStats("unallocated_unused"),
			},
		},
	}

	// test some edge conditions
	assert.Empty(t, c.computeAllocatedDeviceGroupStats(nil, nil))
	assert.Empty(t, c.computeAllocatedDeviceGroupStats(nil, hostDeviceGroupStats))
	assert.Empty(t, c.computeAllocatedDeviceGroupStats(allocatedDevices, nil))

	// actual test
	result := c.computeAllocatedDeviceGroupStats(allocatedDevices, hostDeviceGroupStats)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Vendor < result[j].Vendor
	})

	expected := []*device.DeviceGroupStats{
		{
			Vendor: "vendor",
			Type:   "type",
			Name:   "name",
			InstanceStats: map[string]*device.DeviceStats{
				"d2": newDeviceStats("d2"),
				"d3": newDeviceStats("d3"),
			},
		},
		{
			Vendor: "vendor2",
			Type:   "type2",
			Name:   "name2",
			InstanceStats: map[string]*device.DeviceStats{
				"a2": newDeviceStats("a2"),
			},
		},
	}

	assert.EqualValues(t, expected, result)
}

func TestClient_getAllocatedResources(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	allocStops := mock.BatchAlloc()
	allocStops.Job.TaskGroups[0].Count = 1
	allocStops.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	allocStops.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":   "1ms",
		"exit_code": "0",
	}
	allocStops.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	allocStops.AllocatedResources.Shared.DiskMB = 64
	allocStops.AllocatedResources.Tasks["web"].Cpu = structs.AllocatedCpuResources{CpuShares: 64}
	allocStops.AllocatedResources.Tasks["web"].Memory = structs.AllocatedMemoryResources{MemoryMB: 64}
	require.Nil(client.addAlloc(allocStops, ""))

	allocFails := mock.BatchAlloc()
	allocFails.Job.TaskGroups[0].Count = 1
	allocFails.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	allocFails.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":   "1ms",
		"exit_code": "1",
	}
	allocFails.Job.TaskGroups[0].RestartPolicy.Attempts = 0
	allocFails.AllocatedResources.Shared.DiskMB = 128
	allocFails.AllocatedResources.Tasks["web"].Cpu = structs.AllocatedCpuResources{CpuShares: 128}
	allocFails.AllocatedResources.Tasks["web"].Memory = structs.AllocatedMemoryResources{MemoryMB: 128}
	require.Nil(client.addAlloc(allocFails, ""))

	allocRuns := mock.Alloc()
	allocRuns.Job.TaskGroups[0].Count = 1
	allocRuns.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	allocRuns.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "3s",
	}
	allocRuns.AllocatedResources.Shared.DiskMB = 256
	allocRuns.AllocatedResources.Tasks["web"].Cpu = structs.AllocatedCpuResources{CpuShares: 256}
	allocRuns.AllocatedResources.Tasks["web"].Memory = structs.AllocatedMemoryResources{MemoryMB: 256}
	require.Nil(client.addAlloc(allocRuns, ""))

	allocPends := mock.Alloc()
	allocPends.Job.TaskGroups[0].Count = 1
	allocPends.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	allocPends.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":         "5s",
		"start_block_for": "10s",
	}
	allocPends.AllocatedResources.Shared.DiskMB = 512
	allocPends.AllocatedResources.Tasks["web"].Cpu = structs.AllocatedCpuResources{CpuShares: 512}
	allocPends.AllocatedResources.Tasks["web"].Memory = structs.AllocatedMemoryResources{MemoryMB: 512}
	require.Nil(client.addAlloc(allocPends, ""))

	// wait for allocStops to stop running and for allocRuns to be pending/running
	testutil.WaitForResult(func() (bool, error) {
		as, err := client.GetAllocState(allocPends.ID)
		if err != nil {
			return false, err
		} else if as.ClientStatus != structs.AllocClientStatusPending {
			return false, fmt.Errorf("allocPends not yet pending: %#v", as)
		}

		as, err = client.GetAllocState(allocRuns.ID)
		if as.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("allocRuns not yet running: %#v", as)
		} else if err != nil {
			return false, err
		}

		as, err = client.GetAllocState(allocStops.ID)
		if err != nil {
			return false, err
		} else if as.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("allocStops not yet complete: %#v", as)
		}

		as, err = client.GetAllocState(allocFails.ID)
		if err != nil {
			return false, err
		} else if as.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("allocFails not yet failed: %#v", as)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	result := client.getAllocatedResources(client.config.Node)

	// Ignore comparing networks for now
	result.Flattened.Networks = nil

	expected := structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Cpu: structs.AllocatedCpuResources{
				CpuShares:     768,
				ReservedCores: []uint16{},
			},
			Memory: structs.AllocatedMemoryResources{
				MemoryMB:    768,
				MemoryMaxMB: 768,
			},
			Networks: nil,
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 768,
		},
	}

	assert.EqualValues(t, expected, *result)
}

func TestClient_updateNodeFromDriverUpdatesAll(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	// initial update
	{
		info := &structs.DriverInfo{
			Detected:          true,
			Healthy:           false,
			HealthDescription: "not healthy at start",
			Attributes: map[string]string{
				"node.mock.testattr1": "val1",
			},
		}
		client.updateNodeFromDriver("mock", info)
		n := client.config.Node

		updatedInfo := *n.Drivers["mock"]
		// compare without update time
		updatedInfo.UpdateTime = info.UpdateTime
		assert.EqualValues(t, updatedInfo, *info)

		// check node attributes
		assert.Equal(t, "val1", n.Attributes["node.mock.testattr1"])
	}

	// initial update
	{
		info := &structs.DriverInfo{
			Detected:          true,
			Healthy:           true,
			HealthDescription: "healthy",
			Attributes: map[string]string{
				"node.mock.testattr1": "val2",
			},
		}
		client.updateNodeFromDriver("mock", info)
		n := client.Node()

		updatedInfo := *n.Drivers["mock"]
		// compare without update time
		updatedInfo.UpdateTime = info.UpdateTime
		assert.EqualValues(t, updatedInfo, *info)

		// check node attributes are updated
		assert.Equal(t, "val2", n.Attributes["node.mock.testattr1"])

		// update once more with the same info, updateTime shouldn't change
		client.updateNodeFromDriver("mock", info)
		un := client.Node()
		assert.EqualValues(t, n, un)
	}

	// update once more to unhealthy because why not
	{
		info := &structs.DriverInfo{
			Detected:          true,
			Healthy:           false,
			HealthDescription: "lost track",
			Attributes: map[string]string{
				"node.mock.testattr1": "",
			},
		}
		client.updateNodeFromDriver("mock", info)
		n := client.Node()

		updatedInfo := *n.Drivers["mock"]
		// compare without update time
		updatedInfo.UpdateTime = info.UpdateTime
		assert.EqualValues(t, updatedInfo, *info)

		// check node attributes are updated
		assert.Equal(t, "", n.Attributes["node.mock.testattr1"])

		// update once more with the same info, updateTime shouldn't change
		client.updateNodeFromDriver("mock", info)
		un := client.Node()
		assert.EqualValues(t, n, un)
	}
}

// COMPAT(0.12): remove once upgrading from 0.9.5 is no longer supported
func TestClient_hasLocalState(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	c.stateDB = cstate.NewMemDB(c.logger)

	t.Run("plain alloc", func(t *testing.T) {
		alloc := mock.BatchAlloc()
		c.stateDB.PutAllocation(alloc)

		require.False(t, c.hasLocalState(alloc))
	})

	t.Run("alloc with a task with local state", func(t *testing.T) {
		alloc := mock.BatchAlloc()
		taskName := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0].Name
		ls := &trstate.LocalState{}

		c.stateDB.PutAllocation(alloc)
		c.stateDB.PutTaskRunnerLocalState(alloc.ID, taskName, ls)

		require.True(t, c.hasLocalState(alloc))
	})

	t.Run("alloc with a task with task state", func(t *testing.T) {
		alloc := mock.BatchAlloc()
		taskName := alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0].Name
		ts := &structs.TaskState{
			State: structs.TaskStateRunning,
		}

		c.stateDB.PutAllocation(alloc)
		c.stateDB.PutTaskState(alloc.ID, taskName, ts)

		require.True(t, c.hasLocalState(alloc))
	})
}

func Test_verifiedTasks(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	// produce a result and check against expected tasks and/or error output
	try := func(t *testing.T, a *structs.Allocation, tasks, expTasks []string, expErr string) {
		result, err := verifiedTasks(logger, a, tasks)
		if expErr != "" {
			require.EqualError(t, err, expErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, expTasks, result)
		}
	}

	// create an alloc with TaskGroup=g1, tasks configured given g1Tasks
	alloc := func(g1Tasks []string) *structs.Allocation {
		var tasks []*structs.Task
		for _, taskName := range g1Tasks {
			tasks = append(tasks, &structs.Task{Name: taskName})
		}

		return &structs.Allocation{
			Job: &structs.Job{
				TaskGroups: []*structs.TaskGroup{
					{Name: "g0", Tasks: []*structs.Task{{Name: "g0t1"}}},
					{Name: "g1", Tasks: tasks},
				},
			},
			TaskGroup: "g1",
		}
	}

	t.Run("nil alloc", func(t *testing.T) {
		tasks := []string{"g1t1"}
		try(t, nil, tasks, nil, "nil allocation")
	})

	t.Run("missing task names", func(t *testing.T) {
		var tasks []string
		tgTasks := []string{"g1t1"}
		try(t, alloc(tgTasks), tasks, nil, "missing task names")
	})

	t.Run("missing group", func(t *testing.T) {
		tasks := []string{"g1t1"}
		a := alloc(tasks)
		a.TaskGroup = "other"
		try(t, a, tasks, nil, "group name in allocation is not present in job")
	})

	t.Run("nonexistent task", func(t *testing.T) {
		tasks := []string{"missing"}
		try(t, alloc([]string{"task1"}), tasks, nil, `task "missing" not found in allocation`)
	})

	t.Run("matching task", func(t *testing.T) {
		tasks := []string{"g1t1"}
		try(t, alloc(tasks), tasks, tasks, "")
	})

	t.Run("matching task subset", func(t *testing.T) {
		tasks := []string{"g1t1", "g1t3"}
		tgTasks := []string{"g1t1", "g1t2", "g1t3"}
		try(t, alloc(tgTasks), tasks, tasks, "")
	})
}

func TestClient_ReconnectAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	defer cleanupC1()

	waitTilNodeReady(c1, t)

	job := mock.Job()

	runningAlloc := mock.Alloc()
	runningAlloc.NodeID = c1.Node().ID
	runningAlloc.Job = job
	runningAlloc.JobID = job.ID
	runningAlloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	runningAlloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}
	runningAlloc.ClientStatus = structs.AllocClientStatusPending

	state := s1.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	require.NoError(t, err)

	err = state.UpsertJobSummary(101, mock.JobSummary(runningAlloc.JobID))
	require.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{runningAlloc})
	require.NoError(t, err)

	// Ensure allocation gets upserted with desired status.
	testutil.WaitForResult(func() (bool, error) {
		upsertResult, stateErr := state.AllocByID(nil, runningAlloc.ID)
		return upsertResult.ClientStatus == structs.AllocClientStatusRunning, stateErr
	}, func(err error) {
		require.NoError(t, err, "allocation query failed")
	})

	// Create the unknown version of the alloc from the running one, update state
	// to simulate what reconciler would have done, and then send to the client.
	unknownAlloc, err := state.AllocByID(nil, runningAlloc.ID)
	require.Equal(t, structs.AllocClientStatusRunning, unknownAlloc.ClientStatus)
	require.NoError(t, err)
	unknownAlloc.ClientStatus = structs.AllocClientStatusUnknown
	unknownAlloc.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, runningAlloc.AllocModifyIndex+1, []*structs.Allocation{unknownAlloc})
	require.NoError(t, err)

	updates := &allocUpdates{
		pulled: map[string]*structs.Allocation{
			unknownAlloc.ID: unknownAlloc,
		},
	}

	c1.runAllocs(updates)

	invalid := false
	var runner interfaces.AllocRunner
	var finalAlloc *structs.Allocation
	// Ensure the allocation is not invalid on the client and has been marked
	// running on the server with the new modify index
	testutil.WaitForResult(func() (result bool, stateErr error) {
		c1.allocLock.RLock()
		runner = c1.allocs[unknownAlloc.ID]
		_, invalid = c1.invalidAllocs[unknownAlloc.ID]
		c1.allocLock.RUnlock()

		finalAlloc, stateErr = state.AllocByID(nil, unknownAlloc.ID)
		result = structs.AllocClientStatusRunning == finalAlloc.ClientStatus
		return
	}, func(err error) {
		require.NoError(t, err, "allocation server check failed")
	})

	require.NotNil(t, runner, "expected alloc runner")
	require.False(t, invalid, "expected alloc to not be marked invalid")
	require.Equal(t, unknownAlloc.AllocModifyIndex, finalAlloc.AllocModifyIndex)
}

// TestClient_AllocPrerunErrorDuringRestore ensures that a running allocation,
// which fails Prerun during Restore on client restart, should be killed.
func TestClient_AllocPrerunErrorDuringRestore(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// set up server
	server, _, cleanS1 := testServer(t, nil)
	t.Cleanup(cleanS1)
	testutil.WaitForLeader(t, server.RPC)

	// set up first client, which will initially start the job cleanly
	c1, cleanC1 := TestClient(t, func(c *config.Config) {
		c.DevMode = false // so state persists to client 2
		c.RPCHandler = server
	})
	t.Cleanup(func() {
		test.NoError(t, cleanC1())
	})
	waitTilNodeReady(c1, t)

	// register a happy job to run until we cause it to fail
	job := mock.MinJob()
	testutil.RegisterJob(t, server.RPC, job)

	// wait for our alloc to be running
	testutil.WaitForJobAllocStatus(t, server.RPC, job, map[string]int{
		structs.AllocClientStatusRunning: 1,
	})
	t.Logf("job %s allocs running 👍", job.ID)

	// stop client 1, shutdown will dump state to disk but leave allocs running
	must.NoError(t, c1.Shutdown())

	// make a new client, using parts from the old one to be able to restore state
	restoreClient := func() {
		conf := c1.config.Copy()
		// we want the prerun hook to fail
		hook := allocrunner.NewFailHook(logger, t.Name())
		hook.Fail.Prerun = true
		conf.ExtraAllocHooks = []interfaces.RunnerHook{hook}

		// this is so in-memory driver handles from client 1 can be restored by client 2
		conf.PluginSingletonLoader = singleton.NewSingletonLoader(logger, c1.config.PluginLoader)

		// actually make and start the client
		c2, err := NewClient(conf, c1.consulCatalog, nil, c1.consulService, nil)
		must.NoError(t, err)
		t.Cleanup(func() {
			test.NoError(t, c2.Shutdown())
		})
	}
	restoreClient()

	// wait for the client to pick up the alloc and fail prerun hook
	testutil.WaitForJobAllocStatus(t, server.RPC, job, map[string]int{
		structs.AllocClientStatusFailed: 1,
	})
	t.Logf("job %s allocs failed 👍", job.ID)

	// ok, final assertions
	allocs, err := server.State().AllocsByJob(nil, job.Namespace, job.ID, true)
	must.NoError(t, err)

	ts := allocs[0].TaskStates["t"]
	test.True(t, ts.Failed)
	test.Eq(t, structs.TaskStateDead, ts.State)

	expectEvents := []string{
		// initial successful setup
		structs.TaskReceived,
		structs.TaskSetup,
		structs.TaskStarted,
		// after prerun error during restore
		structs.TaskSetupFailure,
		structs.TaskTerminated, // this whole test is to ensure this happens.
	}
	var actual []string
	for _, event := range ts.Events {
		actual = append(actual, event.Type)
	}
	must.Eq(t, expectEvents, actual)
	test.StrContains(t, ts.Events[3].DisplayMessage, allocrunner.ErrFailHookError.Error())
}
