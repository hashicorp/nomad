package agent

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tmpDir(t testing.TB) string {
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func TestAgent_RPC_Ping(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent(t, t.Name(), nil)
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
	conf.Server.Enabled = true
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
		t.Fatalf("bootstrap expect should be 0")
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
		if expected := "/v1/agent/health?type=client"; check.Path != expected {
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

	t.Run("HTTPS", func(t *testing.T) {
		a := agent()
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

	t.Run("HTTPS + VerifyHTTPSClient", func(t *testing.T) {
		a := agent()
		a.config.TLSConfig.EnableHTTP = true
		a.config.TLSConfig.VerifyHTTPSClient = true

		if check := a.agentHTTPCheck(false); check != nil {
			t.Fatalf("expected nil check not: %#v", check)
		}
	})
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

	// Assert server check uses /v1/agent/health?type=server
	isServer := true
	check := a.agentHTTPCheck(isServer)
	if expected := "Nomad Server HTTP Check"; check.Name != expected {
		t.Errorf("expected server check name to be %q but found %q", expected, check.Name)
	}
	if expected := "/v1/agent/health?type=server"; check.Path != expected {
		t.Errorf("expected server check path to be %q but found %q", expected, check.Path)
	}

	// Assert client check uses /v1/agent/health?type=client
	isServer = false
	check = a.agentHTTPCheck(isServer)
	if expected := "Nomad Client HTTP Check"; check.Name != expected {
		t.Errorf("expected client check name to be %q but found %q", expected, check.Name)
	}
	if expected := "/v1/agent/health?type=client"; check.Path != expected {
		t.Errorf("expected client check path to be %q but found %q", expected, check.Path)
	}
}

// This test asserts that the keyloader embedded in the TLS config is shared
// across the Agent, Server, and Client. This is essential for certificate
// reloading to work.
func TestServer_Reload_TLS_Shared_Keyloader(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// We will start out with a bad cert and then reload with a good one.
	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
		foocert2 = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey2  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer agent.Shutdown()

	originalKeyloader := agent.Config.TLSConfig.GetKeyLoader()
	originalCert, err := originalKeyloader.GetOutgoingCertificate(nil)
	assert.NotNil(originalKeyloader)
	if assert.Nil(err) {
		assert.NotNil(originalCert)
	}

	// Switch to the correct certificates and reload
	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert2,
			KeyFile:              fookey2,
		},
	}

	assert.Nil(agent.Reload(newConfig))
	assert.Equal(agent.Config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.Equal(agent.Config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
	assert.Equal(agent.Config.TLSConfig.GetKeyLoader(), originalKeyloader)

	// Assert is passed through on the server correctly
	if assert.NotNil(agent.server.GetConfig().TLSConfig) {
		serverKeyloader := agent.server.GetConfig().TLSConfig.GetKeyLoader()
		assert.Equal(serverKeyloader, originalKeyloader)
		newCert, err := serverKeyloader.GetOutgoingCertificate(nil)
		assert.Nil(err)
		assert.NotEqual(originalCert, newCert)
	}

	// Assert is passed through on the client correctly
	if assert.NotNil(agent.client.GetConfig().TLSConfig) {
		clientKeyloader := agent.client.GetConfig().TLSConfig.GetKeyLoader()
		assert.Equal(clientKeyloader, originalKeyloader)
		newCert, err := clientKeyloader.GetOutgoingCertificate(nil)
		assert.Nil(err)
		assert.NotEqual(originalCert, newCert)
	}
}

func TestServer_Reload_TLS_Certificate(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
		foocert2 = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey2  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := &Agent{
		config: agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert2,
			KeyFile:              fookey2,
		},
	}

	originalKeyloader := agentConfig.TLSConfig.GetKeyLoader()
	assert.NotNil(originalKeyloader)

	err := agent.Reload(newConfig)
	assert.Nil(err)
	assert.Equal(agent.config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.Equal(agent.config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
	assert.Equal(agent.config.TLSConfig.GetKeyLoader(), originalKeyloader)
}

func TestServer_Reload_TLS_Certificate_Invalid(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
		foocert2 = "invalid_cert_path"
		fookey2  = "invalid_key_path"
	)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := &Agent{
		config: agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert2,
			KeyFile:              fookey2,
		},
	}

	err := agent.Reload(newConfig)
	assert.NotNil(err)
	assert.NotEqual(agent.config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.NotEqual(agent.config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
}

func Test_GetConfig(t *testing.T) {
	assert := assert.New(t)

	agentConfig := &Config{
		Telemetry:      &Telemetry{},
		Client:         &ClientConfig{},
		Server:         &ServerConfig{},
		ACL:            &ACLConfig{},
		Ports:          &Ports{},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		Vault:          &sconfig.VaultConfig{},
		Consul:         &sconfig.ConsulConfig{},
		Sentinel:       &sconfig.SentinelConfig{},
	}

	agent := &Agent{
		config: agentConfig,
	}

	actualAgentConfig := agent.GetConfig()
	assert.Equal(actualAgentConfig, agentConfig)
}

func TestServer_Reload_TLS_WithNilConfiguration(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	logger := log.New(ioutil.Discard, "", 0)

	agent := &Agent{
		logger: logger,
		config: &Config{},
	}

	err := agent.Reload(nil)
	assert.NotNil(err)
	assert.Equal(err.Error(), "cannot reload agent with nil configuration")
}

func TestServer_Reload_TLS_UpgradeToTLS(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	logger := log.New(ioutil.Discard, "", 0)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{},
	}

	agent := &Agent{
		logger: logger,
		config: agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	err := agent.Reload(newConfig)
	assert.Nil(err)

	assert.Equal(agent.config.TLSConfig.CAFile, newConfig.TLSConfig.CAFile)
	assert.Equal(agent.config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.Equal(agent.config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
}

func TestServer_Reload_TLS_DowngradeFromTLS(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	logger := log.New(ioutil.Discard, "", 0)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := &Agent{
		logger: logger,
		config: agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{},
	}

	assert.False(agentConfig.TLSConfig.IsEmpty())

	err := agent.Reload(newConfig)
	assert.Nil(err)

	assert.True(agentConfig.TLSConfig.IsEmpty())
}

func TestServer_ShouldReload_ReturnFalseForNoChanges(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	sameAgentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer agent.Shutdown()

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(sameAgentConfig)
	assert.False(shouldReloadAgent)
	assert.False(shouldReloadHTTP)
	assert.False(shouldReloadRPC)
}

func TestServer_ShouldReload_ReturnTrueForOnlyHTTPChanges(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	sameAgentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           false,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer agent.Shutdown()

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(sameAgentConfig)
	require.True(shouldReloadAgent)
	require.True(shouldReloadHTTP)
	require.False(shouldReloadRPC)
}

func TestServer_ShouldReload_ReturnTrueForOnlyRPCChanges(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	sameAgentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            false,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer agent.Shutdown()

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(sameAgentConfig)
	assert.True(shouldReloadAgent)
	assert.False(shouldReloadHTTP)
	assert.True(shouldReloadRPC)
}

func TestServer_ShouldReload_ReturnTrueForConfigChanges(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
		foocert2 = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey2  = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		}
	})
	defer agent.Shutdown()

	newConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert2,
			KeyFile:              fookey2,
		},
	}

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(newConfig)
	assert.True(shouldReloadAgent)
	assert.True(shouldReloadHTTP)
	assert.True(shouldReloadRPC)
}

func TestServer_ShouldReload_ReturnTrueForFileChanges(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	oldCertificate := `
	-----BEGIN CERTIFICATE-----
	MIICrzCCAlagAwIBAgIUN+4rYZ6wqQCIBzYYd0sfX2e8hDowCgYIKoZIzj0EAwIw
	eDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNh
	biBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwGA1UECxMFTm9tYWQx
	GDAWBgNVBAMTD25vbWFkLmhhc2hpY29ycDAgFw0xNjExMTAxOTU2MDBaGA8yMTE2
	MTAxNzE5NTYwMFoweDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWEx
	FjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwG
	A1UECxMFTm9tYWQxGDAWBgNVBAMTD3JlZ2lvbkZvby5ub21hZDBZMBMGByqGSM49
	AgEGCCqGSM49AwEHA0IABOqGSFNjm+EBlLYlxmIP6SQTdX8U/6hbPWObB0ffkEO/
	CFweeYIVWb3FKNPqYAlhMqg1K0ileD0FbhEzarP0sL6jgbswgbgwDgYDVR0PAQH/
	BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8E
	AjAAMB0GA1UdDgQWBBQnMcjU4yI3k0AoMtapACpO+w9QMTAfBgNVHSMEGDAWgBQ6
	NWr8F5y2eFwqfoQdQPg0kWb9QDA5BgNVHREEMjAwghZzZXJ2ZXIucmVnaW9uRm9v
	Lm5vbWFkghZjbGllbnQucmVnaW9uRm9vLm5vbWFkMAoGCCqGSM49BAMCA0cAMEQC
	ICrvzc5NzqhdT/HkazAx5OOUU8hqoptnmhRmwn6X+0y9AiA8bNvMUxHz3ZLjGBiw
	PLBDC2UaSDqJqiiYpYegLhbQtw==
	-----END CERTIFICATE-----
	`

	content := []byte(oldCertificate)
	dir, err := ioutil.TempDir("", "certificate")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "testcert")
	err = ioutil.WriteFile(tmpfn, content, 0666)
	require.Nil(err)

	const (
		cafile = "../../helper/tlsutil/testdata/ca.pem"
		key    = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	logger := log.New(ioutil.Discard, "", 0)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             tmpfn,
			KeyFile:              key,
		},
	}

	agent := &Agent{
		logger: logger,
		config: agentConfig,
	}
	agent.config.TLSConfig.SetChecksum()

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(agentConfig)
	require.False(shouldReloadAgent)
	require.False(shouldReloadHTTP)
	require.False(shouldReloadRPC)

	newCertificate := `
	-----BEGIN CERTIFICATE-----
	MIICtTCCAlqgAwIBAgIUQp/L2szbgE4b1ASlPOZMReFE27owCgYIKoZIzj0EAwIw
	fDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNh
	biBGcmFuY2lzY28xEjAQBgNVBAoTCUhhc2hpQ29ycDEOMAwGA1UECxMFTm9tYWQx
	HDAaBgNVBAMTE2JhZC5ub21hZC5oYXNoaWNvcnAwIBcNMTYxMTEwMjAxMDAwWhgP
	MjExNjEwMTcyMDEwMDBaMHgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9y
	bmlhMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRIwEAYDVQQKEwlIYXNoaUNvcnAx
	DjAMBgNVBAsTBU5vbWFkMRgwFgYDVQQDEw9yZWdpb25CYWQubm9tYWQwWTATBgcq
	hkjOPQIBBggqhkjOPQMBBwNCAAQk6oXJwlxNrKvl6kpeeR4NJc5EYFI2b3y7odjY
	u55Jp4sI91JVDqnpyatkyGmavdAWa4t0U6HkeaWqKk16/ZcYo4G7MIG4MA4GA1Ud
	DwEB/wQEAwIFoDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0T
	AQH/BAIwADAdBgNVHQ4EFgQUxhzOftFR2L0QAPx8LOuP99WPbpgwHwYDVR0jBBgw
	FoAUHPDLSgzlHqBEh+c4A7HeT0GWygIwOQYDVR0RBDIwMIIWc2VydmVyLnJlZ2lv
	bkJhZC5ub21hZIIWY2xpZW50LnJlZ2lvbkJhZC5ub21hZDAKBggqhkjOPQQDAgNJ
	ADBGAiEAq2rnBeX/St/8i9Cab7Yw0C7pjcaE+mrFYeQByng1Uc0CIQD/o4BrZdkX
	Nm7QGTRZbUFZTHYZr0ULz08Iaz2aHQ6Mcw==
	-----END CERTIFICATE-----
	`

	os.Remove(tmpfn)
	err = ioutil.WriteFile(tmpfn, []byte(newCertificate), 0666)
	require.Nil(err)

	newAgentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             tmpfn,
			KeyFile:              key,
		},
	}

	shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC = agent.ShouldReload(newAgentConfig)
	require.True(shouldReloadAgent)
	require.True(shouldReloadHTTP)
	require.True(shouldReloadRPC)
}

func TestServer_ShouldReload_ShouldHandleMultipleChanges(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
		foocert2 = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey2  = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
	)
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	sameAgentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &sconfig.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert2,
			KeyFile:              fookey2,
		}
	})
	defer agent.Shutdown()

	{
		shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(sameAgentConfig)
		require.True(shouldReloadAgent)
		require.True(shouldReloadHTTP)
		require.True(shouldReloadRPC)
	}

	err := agent.Reload(sameAgentConfig)
	require.Nil(err)

	{
		shouldReloadAgent, shouldReloadHTTP, shouldReloadRPC := agent.ShouldReload(sameAgentConfig)
		require.False(shouldReloadAgent)
		require.False(shouldReloadHTTP)
		require.False(shouldReloadRPC)
	}
}
