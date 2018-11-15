package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/client/config"
	consulApi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/device"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func testACLServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string, *structs.ACLToken) {
	server, token := nomad.TestACLServer(t, cb)
	return server, server.GetConfig().RPCAddr.String(), token
}

func testServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string) {
	server := nomad.TestServer(t, cb)
	return server, server.GetConfig().RPCAddr.String()
}

func TestClient_StartStop(t *testing.T) {
	t.Parallel()
	client, cleanup := TestClient(t, nil)
	defer cleanup()
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Certain labels for metrics are dependant on client initial setup. This tests
// that the client has properly initialized before we assign values to labels
func TestClient_BaseLabels(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	s1, addr := testServer(t, nil)
	defer s1.Shutdown()

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", struct{}{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_RPC_FireRetryWatchers(t *testing.T) {
	t.Parallel()
	s1, addr := testServer(t, nil)
	defer s1.Shutdown()

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	watcher := c1.rpcRetryWatcher()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", struct{}{}, &out)
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
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", struct{}{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_Fingerprint(t *testing.T) {
	t.Parallel()

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

func TestClient_Fingerprint_Periodic(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = map[string]string{
			driver.ShutdownPeriodicAfter:    "true",
			driver.ShutdownPeriodicDuration: "1",
		}
	})
	defer cleanup()

	node := c1.config.Node
	{
		// Ensure the mock driver is registered on the client
		testutil.WaitForResult(func() (bool, error) {
			c1.configLock.Lock()
			defer c1.configLock.Unlock()

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
	}

	{
		testutil.WaitForResult(func() (bool, error) {
			c1.configLock.Lock()
			defer c1.configLock.Unlock()
			mockDriverInfo := node.Drivers["mock_driver"]
			// assert that the Driver information for the node is also set correctly
			if mockDriverInfo == nil {
				return false, fmt.Errorf("mock driver is nil when it should be set on node Drivers")
			}
			if mockDriverInfo.Detected {
				return false, fmt.Errorf("mock driver should be set as detected")
			}
			if mockDriverInfo.Healthy {
				return false, fmt.Errorf("mock driver should be set as healthy")
			}
			if mockDriverInfo.HealthDescription == "" {
				return false, fmt.Errorf("mock driver description should not be empty")
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	}
}

// TestClient_MixedTLS asserts that when a server is running with TLS enabled
// it will reject any RPC connections from clients that lack TLS. See #2525
func TestClient_MixedTLS(t *testing.T) {
	t.Parallel()
	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	s1, addr := testServer(t, func(c *nomad.Config) {
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse
	testutil.AssertUntil(100*time.Millisecond,
		func() (bool, error) {
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil {
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
	t.Parallel()
	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
		badca   = "../helper/tlsutil/testdata/ca-bad.pem"
		badcert = "../helper/tlsutil/testdata/nomad-bad.pem"
		badkey  = "../helper/tlsutil/testdata/nomad-bad-key.pem"
	)
	s1, addr := testServer(t, func(c *nomad.Config) {
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
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
	defer cleanup()

	req := structs.NodeSpecificRequest{
		NodeID:       c1.Node().ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var out structs.SingleNodeResponse
	testutil.AssertUntil(100*time.Millisecond,
		func() (bool, error) {
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil {
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
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

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

func TestClient_Heartbeat(t *testing.T) {
	t.Parallel()
	s1, _ := testServer(t, func(c *nomad.Config) {
		c.MinHeartbeatTTL = 50 * time.Millisecond
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

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

func TestClient_UpdateAllocStatus(t *testing.T) {
	t.Skip("missing exec driver plugin implementation")
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	job := mock.Job()
	alloc := mock.Alloc()
	alloc.NodeID = c1.Node().ID
	alloc.Job = job
	alloc.JobID = job.ID
	originalStatus := "foo"
	alloc.ClientStatus = originalStatus

	// Insert at zero so they are pulled
	state := s1.State()
	if err := state.UpsertJob(0, job); err != nil {
		t.Fatal(err)
	}
	if err := state.UpsertJobSummary(100, mock.JobSummary(alloc.JobID)); err != nil {
		t.Fatal(err)
	}
	state.UpsertAllocs(101, []*structs.Allocation{alloc})

	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.AllocByID(ws, alloc.ID)
		if err != nil {
			return false, err
		}
		if out == nil {
			return false, fmt.Errorf("no such alloc")
		}
		if out.ClientStatus == originalStatus {
			return false, fmt.Errorf("Alloc client status not updated; got %v", out.ClientStatus)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_WatchAllocs(t *testing.T) {
	t.Parallel()
	ctestutil.ExecCompatible(t)
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	// Create mock allocations
	job := mock.Job()
	alloc1 := mock.Alloc()
	alloc1.JobID = job.ID
	alloc1.Job = job
	alloc1.NodeID = c1.Node().ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = c1.Node().ID
	alloc2.JobID = job.ID
	alloc2.Job = job

	state := s1.State()
	if err := state.UpsertJob(100, job); err != nil {
		t.Fatal(err)
	}
	if err := state.UpsertJobSummary(101, mock.JobSummary(alloc1.JobID)); err != nil {
		t.Fatal(err)
	}
	err := state.UpsertAllocs(102, []*structs.Allocation{alloc1, alloc2})
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
	if err := state.DeleteEval(103, nil, []string{alloc1.ID}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the other allocation. Have to make a copy because the allocs are
	// shared in memory in the test and the modify index would be updated in the
	// alloc runner.
	alloc2_2 := alloc2.Copy()
	alloc2_2.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(104, []*structs.Allocation{alloc2_2}); err != nil {
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

func TestClient_SaveRestoreState(t *testing.T) {
	t.Parallel()

	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	defer cleanup()

	// Wait until the node is ready
	waitTilNodeReady(c1, t)

	// Create mock allocations
	job := mock.Job()
	alloc1 := mock.Alloc()
	alloc1.NodeID = c1.Node().ID
	alloc1.Job = job
	alloc1.JobID = job.ID
	alloc1.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	alloc1.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}
	alloc1.ClientStatus = structs.AllocClientStatusRunning

	state := s1.State()
	if err := state.UpsertJob(100, job); err != nil {
		t.Fatal(err)
	}
	if err := state.UpsertJobSummary(101, mock.JobSummary(alloc1.JobID)); err != nil {
		t.Fatal(err)
	}
	if err := state.UpsertAllocs(102, []*structs.Allocation{alloc1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Allocations should get registered
	testutil.WaitForResult(func() (bool, error) {
		c1.allocLock.RLock()
		ar := c1.allocs[alloc1.ID]
		c1.allocLock.RUnlock()
		if ar == nil {
			return false, fmt.Errorf("nil alloc runner")
		}
		if ar.Alloc().ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("client status: got %v; want %v", ar.Alloc().ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Shutdown the client, saves state
	if err := c1.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new client
	logger := testlog.HCLogger(t)
	c1.config.Logger = logger
	catalog := consul.NewMockCatalog(logger)
	mockService := consulApi.NewMockConsulServiceClient(t, logger)
	c2, err := NewClient(c1.config, catalog, mockService)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer c2.Shutdown()

	// Ensure the allocation is running
	testutil.WaitForResult(func() (bool, error) {
		c2.allocLock.RLock()
		ar := c2.allocs[alloc1.ID]
		c2.allocLock.RUnlock()
		status := ar.Alloc().ClientStatus
		alive := status == structs.AllocClientStatusRunning || status == structs.AllocClientStatusPending
		if !alive {
			return false, fmt.Errorf("incorrect client status: %#v", ar.Alloc())
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Destroy all the allocations
	for _, ar := range c2.getAllocRunners() {
		ar.Destroy()
	}

	for _, ar := range c2.getAllocRunners() {
		<-ar.WaitCh()
	}
}

func TestClient_Init(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)
	allocDir := filepath.Join(dir, "alloc")

	client := &Client{
		config: &config.Config{
			AllocDir: allocDir,
		},
		logger: testlog.HCLogger(t),
	}
	if err := client.init(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := os.Stat(allocDir); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestClient_BlockedAllocations(t *testing.T) {
	t.Skip("missing mock driver plugin implementation")
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
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
		"exit_err":    "",
	}

	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(100, []*structs.Allocation{alloc})

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
	if err := state.UpsertAllocs(200, []*structs.Allocation{alloc2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Enusre that the chained allocation is being tracked as blocked
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
	if err := state.UpsertAllocs(300, []*structs.Allocation{alloc1}); err != nil {
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
		<-ar.WaitCh()
	}
}

func TestClient_ValidateMigrateToken_ValidToken(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	assert := assert.New(t)

	c, cleanup := TestClient(t, func(c *config.Config) {})
	defer cleanup()

	assert.Equal(c.ValidateMigrateToken("", ""), true)
}

func TestClient_ReloadTLS_UpgradePlaintextToTLS(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	s1, addr := testServer(t, func(c *nomad.Config) {
		c.Region = "regionFoo"
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer cleanup()

	// Registering a node over plaintext should succeed
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "regionFoo"},
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
		CertFile:             foocert,
		KeyFile:              fookey,
	}

	err := c1.reloadTLSConnections(newConfig)
	assert.Nil(err)

	// Registering a node over plaintext should fail after the node has upgraded
	// to TLS
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "regionFoo"},
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
	t.Parallel()
	assert := assert.New(t)

	s1, addr := testServer(t, func(c *nomad.Config) {
		c.Region = "regionFoo"
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	const (
		cafile  = "../helper/tlsutil/testdata/ca.pem"
		foocert = "../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.TLSConfig = &nconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer cleanup()

	// assert that when one node is running in encrypted mode, a RPC request to a
	// node running in plaintext mode should fail
	{
		req := structs.NodeSpecificRequest{
			NodeID:       c1.Node().ID,
			QueryOptions: structs.QueryOptions{Region: "regionFoo"},
		}
		testutil.WaitForResult(func() (bool, error) {
			var out structs.SingleNodeResponse
			err := c1.RPC("Node.GetNode", &req, &out)
			if err == nil {
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
			QueryOptions: structs.QueryOptions{Region: "regionFoo"},
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
	t.Parallel()
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
	t.Parallel()
	client, cleanup := TestClient(t, func(c *config.Config) {})
	defer cleanup()

	client.updateNodeFromFingerprint(&cstructs.FingerprintResponse{
		NodeResources: &structs.NodeResources{
			Cpu: structs.NodeCpuResources{CpuShares: 123},
		},
	})

	client.updateNodeFromFingerprint(&cstructs.FingerprintResponse{
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
	expectedResources := &structs.NodeResources{
		// computed through test client initialization
		Networks: client.configCopy.Node.NodeResources.Networks,
		Disk:     client.configCopy.Node.NodeResources.Disk,

		// injected
		Cpu:    structs.NodeCpuResources{CpuShares: 123},
		Memory: structs.NodeMemoryResources{MemoryMB: 1024},
		Devices: []*structs.NodeDeviceResource{
			{
				Vendor: "vendor",
				Type:   "type",
			},
		},
	}

	assert.EqualValues(t, expectedResources, client.configCopy.Node.NodeResources)

	// overrides of values

	client.updateNodeFromFingerprint(&cstructs.FingerprintResponse{
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

	expectedResources2 := &structs.NodeResources{
		// computed through test client initialization
		Networks: client.configCopy.Node.NodeResources.Networks,
		Disk:     client.configCopy.Node.NodeResources.Disk,

		// injected
		Cpu:    structs.NodeCpuResources{CpuShares: 123},
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

	assert.EqualValues(t, expectedResources2, client.configCopy.Node.NodeResources)

}

func TestClient_computeAllocatedDeviceStats(t *testing.T) {
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

	require.EqualValues(t, expected, result)
}
