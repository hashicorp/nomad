package agent

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/assert"
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

func TestAgent_RPCPing(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent(t.Name(), nil)
	defer agent.Shutdown()

	var out struct{}
	if err := agent.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ServerConfig(t *testing.T) {
	t.Parallel()
	conf := DefaultConfig()
	conf.DevMode = true // allow localhost for advertise addrs
	a := &Agent{config: conf}

	conf.AdvertiseAddrs.Serf = "127.0.0.1:4000"
	conf.AdvertiseAddrs.RPC = "127.0.0.1:4001"
	conf.AdvertiseAddrs.HTTP = "10.10.11.1:4005"
	conf.ACL.Enabled = true

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
	if out.AuthoritativeRegion != "global" {
		t.Fatalf("bad: %#v", out.AuthoritativeRegion)
	}
	if !out.ACLEnabled {
		t.Fatalf("ACL not enabled")
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

	conf.Server.HeartbeatGrace = 37 * time.Second
	out, err = a.serverConfig()
	if threshold := out.HeartbeatGrace; threshold != time.Second*37 {
		t.Fatalf("expect 37s, got: %s", threshold)
	}

	conf.Server.MinHeartbeatTTL = 37 * time.Second
	out, err = a.serverConfig()
	if min := out.MinHeartbeatTTL; min != time.Second*37 {
		t.Fatalf("expect 37s, got: %s", min)
	}

	conf.Server.MaxHeartbeatsPerSecond = 11.0
	out, err = a.serverConfig()
	if max := out.MaxHeartbeatsPerSecond; max != 11.0 {
		t.Fatalf("expect 11, got: %v", max)
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
	t.Parallel()
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

// Clients should inherit telemetry configuration
func TestAget_Client_TelemetryConfiguration(t *testing.T) {
	assert := assert.New(t)

	conf := DefaultConfig()
	conf.DevMode = true
	conf.Telemetry.DisableTaggedMetrics = true
	conf.Telemetry.BackwardsCompatibleMetrics = true

	a := &Agent{config: conf}

	c, err := a.clientConfig()
	assert.Nil(err)

	telemetry := conf.Telemetry

	assert.Equal(c.StatsCollectionInterval, telemetry.collectionInterval)
	assert.Equal(c.PublishNodeMetrics, telemetry.PublishNodeMetrics)
	assert.Equal(c.PublishAllocationMetrics, telemetry.PublishAllocationMetrics)
	assert.Equal(c.DisableTaggedMetrics, telemetry.DisableTaggedMetrics)
	assert.Equal(c.BackwardsCompatibleMetrics, telemetry.BackwardsCompatibleMetrics)
}

// TestAgent_HTTPCheck asserts Agent.agentHTTPCheck properly alters the HTTP
// API health check depending on configuration.
func TestAgent_HTTPCheck(t *testing.T) {
	t.Parallel()
	logger := log.New(ioutil.Discard, "", 0)
	if testing.Verbose() {
		logger = log.New(os.Stdout, "[TestAgent_HTTPCheck] ", log.Lshortfile)
	}
	agent := func() *Agent {
		return &Agent{
			logger: logger,
			config: &Config{
				AdvertiseAddrs:  &AdvertiseAddrs{HTTP: "advertise:4646"},
				normalizedAddrs: &Addresses{HTTP: "normalized:4646"},
				Consul: &sconfig.ConsulConfig{
					ChecksUseAdvertise: helper.BoolToPtr(false),
				},
				TLSConfig: &sconfig.TLSConfig{EnableHTTP: false},
			},
		}
	}

	t.Run("Plain HTTP Check", func(t *testing.T) {
		a := agent()
		check := a.agentHTTPCheck(false)
		if check == nil {
			t.Fatalf("expected non-nil check")
		}
		if check.Type != "http" {
			t.Errorf("expected http check not: %q", check.Type)
		}
		if expected := "/v1/agent/servers"; check.Path != expected {
			t.Errorf("expected %q path not: %q", expected, check.Path)
		}
		if check.Protocol != "http" {
			t.Errorf("expected http proto not: %q", check.Protocol)
		}
		if expected := a.config.normalizedAddrs.HTTP; check.PortLabel != expected {
			t.Errorf("expected normalized addr not %q", check.PortLabel)
		}
	})

	t.Run("Plain HTTP + ChecksUseAdvertise", func(t *testing.T) {
		a := agent()
		a.config.Consul.ChecksUseAdvertise = helper.BoolToPtr(true)
		check := a.agentHTTPCheck(false)
		if check == nil {
			t.Fatalf("expected non-nil check")
		}
		if expected := a.config.AdvertiseAddrs.HTTP; check.PortLabel != expected {
			t.Errorf("expected advertise addr not %q", check.PortLabel)
		}
	})

	t.Run("HTTPS + consulSupportsTLSSkipVerify", func(t *testing.T) {
		a := agent()
		a.consulSupportsTLSSkipVerify = true
		a.config.TLSConfig.EnableHTTP = true

		check := a.agentHTTPCheck(false)
		if check == nil {
			t.Fatalf("expected non-nil check")
		}
		if !check.TLSSkipVerify {
			t.Errorf("expected tls skip verify")
		}
		if check.Protocol != "https" {
			t.Errorf("expected https not: %q", check.Protocol)
		}
	})

	t.Run("HTTPS w/o TLSSkipVerify", func(t *testing.T) {
		a := agent()
		a.consulSupportsTLSSkipVerify = false
		a.config.TLSConfig.EnableHTTP = true

		if check := a.agentHTTPCheck(false); check != nil {
			t.Fatalf("expected nil check not: %#v", check)
		}
	})

	t.Run("HTTPS + VerifyHTTPSClient", func(t *testing.T) {
		a := agent()
		a.consulSupportsTLSSkipVerify = true
		a.config.TLSConfig.EnableHTTP = true
		a.config.TLSConfig.VerifyHTTPSClient = true

		if check := a.agentHTTPCheck(false); check != nil {
			t.Fatalf("expected nil check not: %#v", check)
		}
	})
}

func TestAgent_ConsulSupportsTLSSkipVerify(t *testing.T) {
	t.Parallel()
	assertSupport := func(expected bool, blob string) {
		self := map[string]map[string]interface{}{}
		if err := json.Unmarshal([]byte("{"+blob+"}"), &self); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		actual := consulSupportsTLSSkipVerify(self)
		if actual != expected {
			t.Errorf("expected %t but got %t for:\n%s\n", expected, actual, blob)
		}
	}

	// 0.6.4
	assertSupport(false, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 3,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.6.4:26a0ef8c",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "1"
        }}`)

	// 0.7.0
	assertSupport(false, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 4,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.7.0:'a189091",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2"
        }}`)

	// 0.7.2
	assertSupport(true, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 4,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 5,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.7.2:'a9afa0c",
            "dc": "dc1",
            "port": "8300",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2"
        }}`)

	// 0.8.1
	assertSupport(true, `"Member": {
        "Addr": "127.0.0.1",
        "DelegateCur": 4,
        "DelegateMax": 5,
        "DelegateMin": 2,
        "Name": "rusty",
        "Port": 8301,
        "ProtocolCur": 2,
        "ProtocolMax": 5,
        "ProtocolMin": 1,
        "Status": 1,
        "Tags": {
            "build": "0.8.1:'e9ca44d",
            "dc": "dc1",
            "id": "3ddc1b59-460e-a100-1d5c-ce3972122664",
            "port": "8300",
            "raft_vsn": "2",
            "role": "consul",
            "vsn": "2",
            "vsn_max": "3",
            "vsn_min": "2",
            "wan_join_port": "8302"
        }}`)
}

// TestAgent_HTTPCheckPath asserts clients and servers use different endpoints
// for healthchecks.
func TestAgent_HTTPCheckPath(t *testing.T) {
	t.Parallel()
	// Agent.agentHTTPCheck only needs a config and logger
	a := &Agent{
		config: DevConfig(),
		logger: log.New(ioutil.Discard, "", 0),
	}
	if err := a.config.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
	}
	if testing.Verbose() {
		a.logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	// Assert server check uses /v1/status/peers
	isServer := true
	check := a.agentHTTPCheck(isServer)
	if expected := "Nomad Server HTTP Check"; check.Name != expected {
		t.Errorf("expected server check name to be %q but found %q", expected, check.Name)
	}
	if expected := "/v1/status/peers"; check.Path != expected {
		t.Errorf("expected server check path to be %q but found %q", expected, check.Path)
	}

	// Assert client check uses /v1/agent/servers
	isServer = false
	check = a.agentHTTPCheck(isServer)
	if expected := "Nomad Client HTTP Check"; check.Name != expected {
		t.Errorf("expected client check name to be %q but found %q", expected, check.Name)
	}
	if expected := "/v1/agent/servers"; check.Path != expected {
		t.Errorf("expected client check path to be %q but found %q", expected, check.Path)
	}
}
