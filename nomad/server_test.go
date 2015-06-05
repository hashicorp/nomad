package nomad

import (
	"fmt"
	"io/ioutil"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

var nextPort uint32 = 15000

func getPort() int {
	return int(atomic.AddUint32(&nextPort, 1))
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func testServer(t *testing.T, cb func(*Config)) *Server {
	// Setup the default settings
	config := DefaultConfig()
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

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	// Create server
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return server
}

func TestServer_RPC(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()

	var out struct{}
	if err := s1.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}
