package agent

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad"
)

var nextPort uint32 = 17000

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

func makeAgent(t *testing.T, cb func(*Config)) (string, *Agent) {
	dir := tmpDir(t)
	conf := DevConfig()

	// Customize the server configuration
	config := nomad.DefaultConfig()
	conf.NomadConfig = config

	// Only use loopback
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
	config.RaftTimeout = 500 * time.Millisecond

	if cb != nil {
		cb(conf)
	}

	agent, err := NewAgent(conf, os.Stderr)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("err: %v", err)
	}
	return dir, agent
}

func TestAgent_RPCPing(t *testing.T) {
	dir, agent := makeAgent(t, nil)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	var out struct{}
	if err := agent.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}
