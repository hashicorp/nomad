package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/hashstructure"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func getPort() int {
	return 1030 + int(rand.Int31n(6440))
}

func testServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string) {
	// Setup the default settings
	config := nomad.DefaultConfig()
	config.VaultConfig.Enabled = helper.BoolToPtr(false)
	config.Build = "unittest"
	config.DevMode = true

	// Tighten the Serf timing
	config.SerfConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfConfig.MemberlistConfig.SuspicionMult = 2
	config.SerfConfig.MemberlistConfig.RetransmitMult = 2
	config.SerfConfig.MemberlistConfig.ProbeTimeout = 50 * time.Millisecond
	config.SerfConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	config.SerfConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	// Tighten the Raft timing
	config.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	config.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	config.RaftConfig.ElectionTimeout = 40 * time.Millisecond
	config.RaftConfig.StartAsLeader = true
	config.RaftTimeout = 500 * time.Millisecond

	logger := log.New(config.LogOutput, "", log.LstdFlags)
	catalog := consul.NewMockCatalog(logger)

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	for i := 10; i >= 0; i-- {
		config.RPCAddr = &net.TCPAddr{
			IP:   []byte{127, 0, 0, 1},
			Port: getPort(),
		}
		config.NodeName = fmt.Sprintf("Node %d", config.RPCAddr.Port)
		config.SerfConfig.MemberlistConfig.BindPort = getPort()

		// Create server
		server, err := nomad.NewServer(config, catalog, logger)
		if err == nil {
			return server, config.RPCAddr.String()
		} else if i == 0 {
			t.Fatalf("err: %v", err)
		} else {
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
		}
	}
	return nil, ""
}

func testClient(t *testing.T, cb func(c *config.Config)) *Client {
	conf := config.DefaultConfig()
	conf.VaultConfig.Enabled = helper.BoolToPtr(false)
	conf.DevMode = true
	conf.Node = &structs.Node{
		Reserved: &structs.Resources{
			DiskMB: 0,
		},
	}

	// Tighten the fingerprinter timeouts
	if conf.Options == nil {
		conf.Options = make(map[string]string)
	}
	conf.Options[fingerprint.TightenNetworkTimeoutsConfig] = "true"

	if cb != nil {
		cb(conf)
	}

	logger := log.New(conf.LogOutput, "", log.LstdFlags)
	catalog := consul.NewMockCatalog(logger)
	mockService := newMockConsulServiceClient()
	mockService.logger = logger
	client, err := NewClient(conf, catalog, mockService, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client
}

func TestClient_StartStop(t *testing.T) {
	t.Parallel()
	client := testClient(t, nil)
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClient_RPC(t *testing.T) {
	t.Parallel()
	s1, addr := testServer(t, nil)
	defer s1.Shutdown()

	c1 := testClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer c1.Shutdown()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", struct{}{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_RPC_Passthrough(t *testing.T) {
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

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
	c := testClient(t, nil)
	defer c.Shutdown()

	// Ensure kernel and arch are always present
	node := c.Node()
	if node.Attributes["kernel.name"] == "" {
		t.Fatalf("missing kernel.name")
	}
	if node.Attributes["cpu.arch"] == "" {
		t.Fatalf("missing cpu arch")
	}
}

func TestClient_HasNodeChanged(t *testing.T) {
	t.Parallel()
	c := testClient(t, nil)
	defer c.Shutdown()

	node := c.Node()
	attrHash, err := hashstructure.Hash(node.Attributes, nil)
	if err != nil {
		c.logger.Printf("[DEBUG] client: unable to calculate node attributes hash: %v", err)
	}
	// Calculate node meta map hash
	metaHash, err := hashstructure.Hash(node.Meta, nil)
	if err != nil {
		c.logger.Printf("[DEBUG] client: unable to calculate node meta hash: %v", err)
	}
	if changed, _, _ := c.hasNodeChanged(attrHash, metaHash); changed {
		t.Fatalf("Unexpected hash change.")
	}

	// Change node attribute
	node.Attributes["arch"] = "xyz_86"
	if changed, newAttrHash, _ := c.hasNodeChanged(attrHash, metaHash); !changed {
		t.Fatalf("Expected hash change in attributes: %d vs %d", attrHash, newAttrHash)
	}

	// Change node meta map
	node.Meta["foo"] = "bar"
	if changed, _, newMetaHash := c.hasNodeChanged(attrHash, metaHash); !changed {
		t.Fatalf("Expected hash change in meta map: %d vs %d", metaHash, newMetaHash)
	}
}

func TestClient_Fingerprint_InWhitelist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Weird spacing to test trimming. Whitelist all modules expect cpu.
		c.Options["fingerprint.whitelist"] = "  arch, consul,cpu,env_aws,env_gce,host,memory,network,storage,foo,bar	"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["cpu.frequency"] == "" {
		t.Fatalf("missing cpu fingerprint module")
	}
}

func TestClient_Fingerprint_InBlacklist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Weird spacing to test trimming. Blacklist cpu.
		c.Options["fingerprint.blacklist"] = "  cpu	"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["cpu.frequency"] != "" {
		t.Fatalf("cpu fingerprint module loaded despite blacklisting")
	}
}

func TestClient_Fingerprint_OutOfWhitelist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		c.Options["fingerprint.whitelist"] = "arch,consul,env_aws,env_gce,host,memory,network,storage,foo,bar"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["cpu.frequency"] != "" {
		t.Fatalf("found cpu fingerprint module")
	}
}

func TestClient_Fingerprint_WhitelistBlacklistCombination(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// With both white- and blacklist, should return the set difference of modules (arch, cpu)
		c.Options["fingerprint.whitelist"] = "arch,memory,cpu"
		c.Options["fingerprint.blacklist"] = "memory,nomad"
	})
	defer c.Shutdown()

	node := c.Node()
	// Check expected modules are present
	if node.Attributes["cpu.frequency"] == "" {
		t.Fatalf("missing cpu fingerprint module")
	}
	if node.Attributes["cpu.arch"] == "" {
		t.Fatalf("missing arch fingerprint module")
	}
	// Check remainder _not_ present
	if node.Attributes["memory.totalbytes"] != "" {
		t.Fatalf("found memory fingerprint module")
	}
	if node.Attributes["nomad.version"] != "" {
		t.Fatalf("found nomad fingerprint module")
	}
}

func TestClient_Drivers_InWhitelist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Weird spacing to test trimming
		c.Options["driver.raw_exec.enable"] = "1"
		c.Options["driver.whitelist"] = "   raw_exec ,  foo	"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.raw_exec"] == "" {
		t.Fatalf("missing raw_exec driver")
	}
}

func TestClient_Drivers_InBlacklist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Weird spacing to test trimming
		c.Options["driver.raw_exec.enable"] = "1"
		c.Options["driver.blacklist"] = "   raw_exec ,  foo	"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.raw_exec"] != "" {
		t.Fatalf("raw_exec driver loaded despite blacklist")
	}
}

func TestClient_Drivers_OutOfWhitelist(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		c.Options["driver.whitelist"] = "foo,bar,baz"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.exec"] != "" {
		t.Fatalf("found exec driver")
	}
}

func TestClient_Drivers_WhitelistBlacklistCombination(t *testing.T) {
	t.Parallel()
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Expected output is set difference (raw_exec)
		c.Options["driver.whitelist"] = "raw_exec,exec"
		c.Options["driver.blacklist"] = "exec"
	})
	defer c.Shutdown()

	node := c.Node()
	// Check expected present
	if node.Attributes["driver.raw_exec"] == "" {
		t.Fatalf("missing raw_exec driver")
	}
	// Check expected absent
	if node.Attributes["driver.exec"] != "" {
		t.Fatalf("exec driver loaded despite blacklist")
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

	c1 := testClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
	})
	defer c1.Shutdown()

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

	c1 := testClient(t, func(c *config.Config) {
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
	defer c1.Shutdown()

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

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

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

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

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
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	// Wait til the node is ready
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

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	// Wait til the node is ready
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

	// Insert at zero so they are pulled
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
	err = state.DeleteEval(103, nil, []string{alloc1.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the other allocation. Have to make a copy because the allocs are
	// shared in memory in the test and the modify index would be updated in the
	// alloc runner.
	alloc2_2 := new(structs.Allocation)
	*alloc2_2 = *alloc2
	alloc2_2.DesiredStatus = structs.AllocDesiredStatusStop
	err = state.UpsertAllocs(104, []*structs.Allocation{alloc2_2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// One allocations should get de-registered
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
	ctestutil.ExecCompatible(t)
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	// Wait til the node is ready
	waitTilNodeReady(c1, t)

	// Create mock allocations
	job := mock.Job()
	alloc1 := mock.Alloc()
	alloc1.NodeID = c1.Node().ID
	alloc1.Job = job
	alloc1.JobID = job.ID
	alloc1.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	task := alloc1.Job.TaskGroups[0].Tasks[0]
	task.Config["run_for"] = "10s"

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
	logger := log.New(c1.config.LogOutput, "", log.LstdFlags)
	catalog := consul.NewMockCatalog(logger)
	mockService := newMockConsulServiceClient()
	mockService.logger = logger
	c2, err := NewClient(c1.config, catalog, mockService, logger)
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
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	if err := client.init(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := os.Stat(allocDir); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestClient_BlockedAllocations(t *testing.T) {
	t.Parallel()
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

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
	alloc2.ID = structs.GenerateUUID()
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
