package agent

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
)

func getPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func tmpDir(t testing.TB) string {
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func makeAgent(t testing.TB, cb func(*Config)) (string, *Agent) {
	dir := tmpDir(t)
	conf := DevConfig()

	// Customize the server configuration
	config := nomad.DefaultConfig()
	conf.NomadConfig = config

	// Bind and set ports
	conf.BindAddr = "127.0.0.1"
	conf.Ports = &Ports{
		HTTP: getPort(),
		RPC:  getPort(),
		Serf: getPort(),
	}
	conf.NodeName = fmt.Sprintf("Node %d", conf.Ports.RPC)
	conf.Consul = sconfig.DefaultConsulConfig()
	conf.Vault.Enabled = false

	// Tighten the Serf timing
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

func TestAgent_ServerConfig(t *testing.T) {
	conf := DefaultConfig()
	a := &Agent{config: conf}

	// Returns error on bad serf addr
	conf.AdvertiseAddrs.Serf = "nope"
	_, err := a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "serf advertise") {
		t.Fatalf("expected serf address error, got: %#v", err)
	}
	conf.AdvertiseAddrs.Serf = "127.0.0.1:4000"

	// Returns error on bad rpc addr
	conf.AdvertiseAddrs.RPC = "nope"
	_, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "rpc advertise") {
		t.Fatalf("expected rpc address error, got: %#v", err)
	}
	conf.AdvertiseAddrs.RPC = "127.0.0.1:4001"
	conf.AdvertiseAddrs.HTTP = "10.10.11.1:4005"

	// Parses the advertise addrs correctly
	out, err := a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	serfAddr := out.SerfConfig.MemberlistConfig.AdvertiseAddr
	if serfAddr != "127.0.0.1" {
		t.Fatalf("expect 127.0.0.1, got: %s", serfAddr)
	}
	serfPort := out.SerfConfig.MemberlistConfig.AdvertisePort
	if serfPort != 4000 {
		t.Fatalf("expected 4000, got: %d", serfPort)
	}
	if addr := out.RPCAdvertise; addr.IP.String() != "127.0.0.1" || addr.Port != 4001 {
		t.Fatalf("bad rpc advertise addr: %#v", addr)
	}
	if addr := a.serverHTTPAddr; addr != "10.10.11.1:4005" {
		t.Fatalf("expect 10.11.11.1:4005, got: %v", addr)
	}
	if addr := a.serverRPCAddr; addr != "127.0.0.1:4001" {
		t.Fatalf("expect 127.0.0.1:4001, got: %v", addr)
	}

	// Sets up the ports properly
	conf.Ports.RPC = 4003
	conf.Ports.Serf = 4004

	out, err = a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if addr := out.RPCAddr.Port; addr != 4003 {
		t.Fatalf("expect 4003, got: %d", out.RPCAddr.Port)
	}
	if port := out.SerfConfig.MemberlistConfig.BindPort; port != 4004 {
		t.Fatalf("expect 4004, got: %d", port)
	}

	// Prefers advertise over bind addr
	conf.BindAddr = "127.0.0.3"
	conf.Addresses.RPC = "127.0.0.2"
	conf.Addresses.Serf = "127.0.0.2"
	conf.Addresses.HTTP = "127.0.0.2"
	conf.AdvertiseAddrs.HTTP = ""

	out, err = a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if addr := out.RPCAddr.IP.String(); addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if addr := out.SerfConfig.MemberlistConfig.BindAddr; addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if addr := a.serverHTTPAddr; addr != "127.0.0.2:4646" {
		t.Fatalf("expect 127.0.0.2:4646, got: %s", addr)
	}
	// NOTE: AdvertiseAddr > Addresses > BindAddr > Defaults
	if addr := a.serverRPCAddr; addr != "127.0.0.1:4001" {
		t.Fatalf("expect 127.0.0.1:4001, got: %s", addr)
	}
	if addr := a.serverSerfAddr; addr != "127.0.0.1:4000" {
		t.Fatalf("expect 127.0.0.1:4000, got: %s", addr)
	}

	conf.Server.NodeGCThreshold = "42g"
	out, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("expected unknown unit error, got: %#v", err)
	}
	conf.Server.NodeGCThreshold = "10s"
	out, err = a.serverConfig()
	if threshold := out.NodeGCThreshold; threshold != time.Second*10 {
		t.Fatalf("expect 10s, got: %s", threshold)
	}

	conf.Server.HeartbeatGrace = "42g"
	out, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("expected unknown unit error, got: %#v", err)
	}
	conf.Server.HeartbeatGrace = "37s"
	out, err = a.serverConfig()
	if threshold := out.HeartbeatGrace; threshold != time.Second*37 {
		t.Fatalf("expect 37s, got: %s", threshold)
	}

	// Defaults to the global bind addr
	conf.Addresses.RPC = ""
	conf.Addresses.Serf = ""
	conf.Addresses.HTTP = ""
	conf.AdvertiseAddrs.RPC = ""
	conf.AdvertiseAddrs.HTTP = ""
	conf.AdvertiseAddrs.Serf = ""
	conf.Ports.HTTP = 4646
	conf.Ports.RPC = 4647
	conf.Ports.Serf = 4648
	out, err = a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if addr := out.RPCAddr.IP.String(); addr != "127.0.0.3" {
		t.Fatalf("expect 127.0.0.3, got: %s", addr)
	}
	if addr := out.SerfConfig.MemberlistConfig.BindAddr; addr != "127.0.0.3" {
		t.Fatalf("expect 127.0.0.3, got: %s", addr)
	}
	if addr := a.serverHTTPAddr; addr != "127.0.0.3:4646" {
		t.Fatalf("expect 127.0.0.3:4646, got: %s", addr)
	}
	if addr := a.serverRPCAddr; addr != "127.0.0.3:4647" {
		t.Fatalf("expect 127.0.0.3:4647, got: %s", addr)
	}
	if addr := a.serverSerfAddr; addr != "127.0.0.3:4648" {
		t.Fatalf("expect 127.0.0.3:4648, got: %s", addr)
	}

	// Properly handles the bootstrap flags
	conf.Server.BootstrapExpect = 1
	out, err = a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !out.Bootstrap {
		t.Fatalf("should have set bootstrap mode")
	}
	if out.BootstrapExpect != 0 {
		t.Fatalf("boostrap expect should be 0")
	}

	conf.Server.BootstrapExpect = 3
	out, err = a.serverConfig()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.Bootstrap {
		t.Fatalf("bootstrap mode should be disabled")
	}
	if out.BootstrapExpect != 3 {
		t.Fatalf("should have bootstrap-expect = 3")
	}
}

func TestAgent_ClientConfig(t *testing.T) {
	conf := DefaultConfig()
	a := &Agent{config: conf}
	conf.Client.Enabled = true
	conf.Addresses.HTTP = "127.0.0.1"
	conf.Ports.HTTP = 5678

	c, err := a.clientConfig()
	if err != nil {
		t.Fatalf("got err: %v", err)
	}

	expectedHttpAddr := "127.0.0.1:5678"
	if c.Node.HTTPAddr != expectedHttpAddr {
		t.Fatalf("Expected http addr: %v, got: %v", expectedHttpAddr, c.Node.HTTPAddr)
	}

	conf = DefaultConfig()
	a = &Agent{config: conf}
	conf.Client.Enabled = true
	conf.Addresses.HTTP = "127.0.0.1"

	c, err = a.clientConfig()
	if err != nil {
		t.Fatalf("got err: %v", err)
	}

	expectedHttpAddr = "127.0.0.1:4646"
	if c.Node.HTTPAddr != expectedHttpAddr {
		t.Fatalf("Expected http addr: %v, got: %v", expectedHttpAddr, c.Node.HTTPAddr)
	}
}
