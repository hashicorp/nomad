package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/hashstructure"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

var nextPort uint32 = 16000

func getPort() int {
	return int(atomic.AddUint32(&nextPort, 1))
}

func testServer(t *testing.T, cb func(*nomad.Config)) (*nomad.Server, string) {
	// Setup the default settings
	config := nomad.DefaultConfig()
	config.Build = "unittest"
	config.DevMode = true
	config.RPCAddr = &net.TCPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: getPort(),
	}
	config.NodeName = fmt.Sprintf("Node %d", config.RPCAddr.Port)

	// Tighten the Serf timing
	config.SerfConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	config.SerfConfig.MemberlistConfig.BindPort = getPort()
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

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	// Create server
	server, err := nomad.NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return server, config.RPCAddr.String()
}

func testClient(t *testing.T, cb func(c *config.Config)) *Client {
	conf := DefaultConfig()
	conf.DevMode = true
	if cb != nil {
		cb(conf)
	}

	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client
}

func TestClient_StartStop(t *testing.T) {
	client := testClient(t, nil)
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClient_RPC(t *testing.T) {
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
	c := testClient(t, nil)
	defer c.Shutdown()

	// Ensure kernel and arch are always present
	node := c.Node()
	if node.Attributes["kernel.name"] == "" {
		t.Fatalf("missing kernel.name")
	}
	if node.Attributes["arch"] == "" {
		t.Fatalf("missing arch")
	}
}

func TestClient_HasNodeChanged(t *testing.T) {
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

func TestClient_Fingerprint_OutOfWhitelist(t *testing.T) {
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

func TestClient_Drivers(t *testing.T) {
	c := testClient(t, nil)
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing exec driver")
	}
}

func TestClient_Drivers_InWhitelist(t *testing.T) {
	c := testClient(t, func(c *config.Config) {
		if c.Options == nil {
			c.Options = make(map[string]string)
		}

		// Weird spacing to test trimming
		c.Options["driver.whitelist"] = "   exec ,  foo	"
	})
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing exec driver")
	}
}

func TestClient_Drivers_OutOfWhitelist(t *testing.T) {
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

func TestClient_Register(t *testing.T) {
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
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	alloc := mock.Alloc()
	alloc.NodeID = c1.Node().ID
	originalStatus := "foo"
	alloc.ClientStatus = originalStatus

	state := s1.State()
	state.UpsertAllocs(100, []*structs.Allocation{alloc})

	testutil.WeightedWaitForResult(4000, func() (bool, error) {
		out, err := state.AllocByID(alloc.ID)
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
	ctestutil.ExecCompatible(t)
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	// Create mock allocations
	alloc1 := mock.Alloc()
	alloc1.NodeID = c1.Node().ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = c1.Node().ID

	state := s1.State()
	err := state.UpsertAllocs(100,
		[]*structs.Allocation{alloc1, alloc2})
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
	err = state.DeleteEval(101, nil, []string{alloc1.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the other allocation. Have to make a copy because the allocs are
	// shared in memory in the test and the modify index would be updated in the
	// alloc runner.
	alloc2_2 := new(structs.Allocation)
	*alloc2_2 = *alloc2
	alloc2_2.DesiredStatus = structs.AllocDesiredStatusStop
	err = state.UpsertAllocs(102,
		[]*structs.Allocation{alloc2_2})
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

func TestClient_SaveRestoreState(t *testing.T) {
	ctestutil.ExecCompatible(t)
	s1, _ := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	c1 := testClient(t, func(c *config.Config) {
		c.DevMode = false
		c.RPCHandler = s1
	})
	defer c1.Shutdown()

	// Create mock allocations
	alloc1 := mock.Alloc()
	alloc1.NodeID = c1.Node().ID
	task := alloc1.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}

	state := s1.State()
	err := state.UpsertAllocs(100, []*structs.Allocation{alloc1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Allocations should get registered
	testutil.WeightedWaitForResult(4000, func() (bool, error) {
		c1.allocLock.RLock()
		ar := c1.allocs[alloc1.ID]
		c1.allocLock.RUnlock()
		return ar != nil && ar.Alloc().ClientStatus == structs.AllocClientStatusRunning, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Shutdown the client, saves state
	err = c1.Shutdown()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new client
	c2, err := NewClient(c1.config)
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
		alive := status != structs.AllocClientStatusRunning ||
			status != structs.AllocClientStatusPending
		if !alive {
			return false, fmt.Errorf("incorrect client status: %#v", ar.Alloc())
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClient_Init(t *testing.T) {
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

func TestClient_SetServers(t *testing.T) {
	client := testClient(t, nil)

	// Sets an empty list
	client.SetServers(nil)
	if client.servers == nil {
		t.Fatalf("should not be nil")
	}

	// Set the initial servers list
	expect := []string{"foo:4647"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Add a server
	expect = []string{"foo:5445", "bar:8080"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Remove a server
	expect = []string{"bar:8080"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Add and remove a server
	expect = []string{"baz:9090", "zip:4545"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Query the servers list
	if servers := client.Servers(); !reflect.DeepEqual(servers, expect) {
		t.Fatalf("expect %v, got %v", expect, servers)
	}

	// Add servers without ports, and remove old servers
	servers := []string{"foo", "bar", "baz"}
	expect = []string{"foo:4647", "bar:4647", "baz:4647"}
	client.SetServers(servers)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}
}
