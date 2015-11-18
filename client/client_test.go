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

func TestClient_Drivers(t *testing.T) {
	ctestutil.ExecCompatible(t)
	c := testClient(t, nil)
	defer c.Shutdown()

	node := c.Node()
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing exec driver")
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

	state := s1.State()
	state.UpsertAllocs(100, []*structs.Allocation{alloc})

	newAlloc := new(structs.Allocation)
	*newAlloc = *alloc
	newAlloc.ClientStatus = structs.AllocClientStatusRunning

	err := c1.updateAllocStatus(newAlloc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out == nil || out.ClientStatus != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", out)
	}
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

	// Update the other allocation
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop
	err = state.UpsertAllocs(102,
		[]*structs.Allocation{alloc2})
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
	err := state.UpsertAllocs(100,
		[]*structs.Allocation{alloc1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Allocations should get registered
	testutil.WaitForResult(func() (bool, error) {
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
	c2.allocLock.RLock()
	ar := c2.allocs[alloc1.ID]
	c2.allocLock.RUnlock()
	if ar.Alloc().ClientStatus != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", ar.Alloc())
	}
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
	expect := []string{"foo"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Add a server
	expect = []string{"foo", "bar"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Remove a server
	expect = []string{"bar"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Add and remove a server
	expect = []string{"baz", "zip"}
	client.SetServers(expect)
	if !reflect.DeepEqual(client.servers, expect) {
		t.Fatalf("expect %v, got %v", expect, client.servers)
	}

	// Query the servers list
	if servers := client.Servers(); !reflect.DeepEqual(servers, expect) {
		t.Fatalf("expect %v, got %v", expect, servers)
	}
}
