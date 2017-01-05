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
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
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

	// Set the data_dir
	conf.DataDir = dir
	conf.NomadConfig.DataDir = dir

	// Bind and set ports
	conf.BindAddr = "127.0.0.1"
	conf.Ports = &Ports{
		HTTP: getPort(),
		RPC:  getPort(),
		Serf: getPort(),
	}
	conf.NodeName = fmt.Sprintf("Node %d", conf.Ports.RPC)
	conf.Consul = sconfig.DefaultConsulConfig()
	conf.Vault.Enabled = new(bool)

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

	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
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
	conf.DevMode = true // allow localhost for advertise addrs
	a := &Agent{config: conf}

	conf.AdvertiseAddrs.Serf = "127.0.0.1:4000"
	conf.AdvertiseAddrs.RPC = "127.0.0.1:4001"
	conf.AdvertiseAddrs.HTTP = "10.10.11.1:4005"

	// Parses the advertise addrs correctly
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
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

	// Assert addresses weren't changed
	if addr := conf.AdvertiseAddrs.RPC; addr != "127.0.0.1:4001" {
		t.Fatalf("bad rpc advertise addr: %#v", addr)
	}
	if addr := conf.AdvertiseAddrs.HTTP; addr != "10.10.11.1:4005" {
		t.Fatalf("expect 10.11.11.1:4005, got: %v", addr)
	}
	if addr := conf.Addresses.RPC; addr != "0.0.0.0" {
		t.Fatalf("expect 0.0.0.0, got: %v", addr)
	}

	// Sets up the ports properly
	conf.Addresses.RPC = ""
	conf.Addresses.Serf = ""
	conf.Ports.RPC = 4003
	conf.Ports.Serf = 4004

	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
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
	conf.Addresses.HTTP = "127.0.0.2"
	conf.Addresses.RPC = "127.0.0.2"
	conf.Addresses.Serf = "127.0.0.2"
	conf.AdvertiseAddrs.HTTP = "10.0.0.10"
	conf.AdvertiseAddrs.RPC = ""
	conf.AdvertiseAddrs.Serf = "10.0.0.12:4004"

	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	out, err = a.serverConfig()
	fmt.Println(conf.Addresses.RPC)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if addr := out.RPCAddr.IP.String(); addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if port := out.RPCAddr.Port; port != 4003 {
		t.Fatalf("expect 4647, got: %d", port)
	}
	if addr := out.SerfConfig.MemberlistConfig.BindAddr; addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if port := out.SerfConfig.MemberlistConfig.BindPort; port != 4004 {
		t.Fatalf("expect 4648, got: %d", port)
	}
	if addr := conf.Addresses.HTTP; addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if addr := conf.Addresses.RPC; addr != "127.0.0.2" {
		t.Fatalf("expect 127.0.0.2, got: %s", addr)
	}
	if addr := conf.Addresses.Serf; addr != "127.0.0.2" {
		t.Fatalf("expect 10.0.0.12, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.HTTP; addr != "127.0.0.2:4646" {
		t.Fatalf("expect 127.0.0.2:4646, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.RPC; addr != "127.0.0.2:4003" {
		t.Fatalf("expect 127.0.0.2:4003, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.Serf; addr != "127.0.0.2:4004" {
		t.Fatalf("expect 10.0.0.12:4004, got: %s", addr)
	}
	if addr := conf.AdvertiseAddrs.HTTP; addr != "10.0.0.10:4646" {
		t.Fatalf("expect 10.0.0.10:4646, got: %s", addr)
	}
	if addr := conf.AdvertiseAddrs.RPC; addr != "127.0.0.2:4003" {
		t.Fatalf("expect 127.0.0.2:4003, got: %s", addr)
	}
	if addr := conf.AdvertiseAddrs.Serf; addr != "10.0.0.12:4004" {
		t.Fatalf("expect 10.0.0.12:4004, got: %s", addr)
	}

	conf.Server.NodeGCThreshold = "42g"
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	out, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("expected unknown unit error, got: %#v", err)
	}

	conf.Server.NodeGCThreshold = "10s"
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	out, err = a.serverConfig()
	if threshold := out.NodeGCThreshold; threshold != time.Second*10 {
		t.Fatalf("expect 10s, got: %s", threshold)
	}

	conf.Server.HeartbeatGrace = "42g"
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	out, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("expected unknown unit error, got: %#v", err)
	}

	conf.Server.HeartbeatGrace = "37s"
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
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
	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
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
	if addr := conf.Addresses.HTTP; addr != "127.0.0.3" {
		t.Fatalf("expect 127.0.0.3, got: %s", addr)
	}
	if addr := conf.Addresses.RPC; addr != "127.0.0.3" {
		t.Fatalf("expect 127.0.0.3, got: %s", addr)
	}
	if addr := conf.Addresses.Serf; addr != "127.0.0.3" {
		t.Fatalf("expect 127.0.0.3, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.HTTP; addr != "127.0.0.3:4646" {
		t.Fatalf("expect 127.0.0.3:4646, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.RPC; addr != "127.0.0.3:4647" {
		t.Fatalf("expect 127.0.0.3:4647, got: %s", addr)
	}
	if addr := conf.normalizedAddrs.Serf; addr != "127.0.0.3:4648" {
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
	conf.Client.Enabled = true

	// For Clients HTTP and RPC must be set (Serf can be skipped)
	conf.Addresses.HTTP = "169.254.0.1"
	conf.Addresses.RPC = "169.254.0.1"
	conf.Ports.HTTP = 5678
	a := &Agent{config: conf}

	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	c, err := a.clientConfig()
	if err != nil {
		t.Fatalf("got err: %v", err)
	}

	expectedHttpAddr := "169.254.0.1:5678"
	if c.Node.HTTPAddr != expectedHttpAddr {
		t.Fatalf("Expected http addr: %v, got: %v", expectedHttpAddr, c.Node.HTTPAddr)
	}

	conf = DefaultConfig()
	conf.DevMode = true
	a = &Agent{config: conf}
	conf.Client.Enabled = true
	conf.Addresses.HTTP = "169.254.0.1"

	if err := conf.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	c, err = a.clientConfig()
	if err != nil {
		t.Fatalf("got err: %v", err)
	}

	expectedHttpAddr = "169.254.0.1:4646"
	if c.Node.HTTPAddr != expectedHttpAddr {
		t.Fatalf("Expected http addr: %v, got: %v", expectedHttpAddr, c.Node.HTTPAddr)
	}
}
