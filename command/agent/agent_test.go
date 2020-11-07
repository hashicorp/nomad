package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
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
	require.NoError(t, err)

	require.True(t, out.EnableEventBroker)

	serfAddr := out.SerfConfig.MemberlistConfig.AdvertiseAddr
	require.Equal(t, "127.0.0.1", serfAddr)

	serfPort := out.SerfConfig.MemberlistConfig.AdvertisePort
	require.Equal(t, 4000, serfPort)

	require.Equal(t, "global", out.AuthoritativeRegion)
	require.True(t, out.ACLEnabled)

	// Assert addresses weren't changed
	require.Equal(t, "127.0.0.1:4001", conf.AdvertiseAddrs.RPC)
	require.Equal(t, "10.10.11.1:4005", conf.AdvertiseAddrs.HTTP)
	require.Equal(t, "0.0.0.0", conf.Addresses.RPC)

	// Sets up the ports properly
	conf.Addresses.RPC = ""
	conf.Addresses.Serf = ""
	conf.Ports.RPC = 4003
	conf.Ports.Serf = 4004

	require.NoError(t, conf.normalizeAddrs())

	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 4003, out.RPCAddr.Port)
	require.Equal(t, 4004, out.SerfConfig.MemberlistConfig.BindPort)

	// Prefers advertise over bind addr
	conf.BindAddr = "127.0.0.3"
	conf.Addresses.HTTP = "127.0.0.2"
	conf.Addresses.RPC = "127.0.0.2"
	conf.Addresses.Serf = "127.0.0.2"
	conf.AdvertiseAddrs.HTTP = "10.0.0.10"
	conf.AdvertiseAddrs.RPC = ""
	conf.AdvertiseAddrs.Serf = "10.0.0.12:4004"

	require.NoError(t, conf.normalizeAddrs())

	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, "127.0.0.2", out.RPCAddr.IP.String())
	require.Equal(t, 4003, out.RPCAddr.Port)
	require.Equal(t, "127.0.0.2", out.SerfConfig.MemberlistConfig.BindAddr)
	require.Equal(t, 4004, out.SerfConfig.MemberlistConfig.BindPort)
	require.Equal(t, "127.0.0.2", conf.Addresses.HTTP)
	require.Equal(t, "127.0.0.2", conf.Addresses.RPC)
	require.Equal(t, "127.0.0.2", conf.Addresses.Serf)
	require.Equal(t, "127.0.0.2:4646", conf.normalizedAddrs.HTTP)
	require.Equal(t, "127.0.0.2:4003", conf.normalizedAddrs.RPC)
	require.Equal(t, "127.0.0.2:4004", conf.normalizedAddrs.Serf)
	require.Equal(t, "10.0.0.10:4646", conf.AdvertiseAddrs.HTTP)
	require.Equal(t, "127.0.0.2:4003", conf.AdvertiseAddrs.RPC)
	require.Equal(t, "10.0.0.12:4004", conf.AdvertiseAddrs.Serf)

	conf.Server.NodeGCThreshold = "42g"
	require.NoError(t, conf.normalizeAddrs())

	_, err = a.serverConfig()
	if err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Fatalf("expected unknown unit error, got: %#v", err)
	}

	conf.Server.NodeGCThreshold = "10s"
	require.NoError(t, conf.normalizeAddrs())
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, out.NodeGCThreshold)

	conf.Server.HeartbeatGrace = 37 * time.Second
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 37*time.Second, out.HeartbeatGrace)

	conf.Server.MinHeartbeatTTL = 37 * time.Second
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 37*time.Second, out.MinHeartbeatTTL)

	conf.Server.MaxHeartbeatsPerSecond = 11.0
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, float64(11.0), out.MaxHeartbeatsPerSecond)

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
	require.NoError(t, conf.normalizeAddrs())

	out, err = a.serverConfig()
	require.NoError(t, err)

	require.Equal(t, "127.0.0.3", out.RPCAddr.IP.String())
	require.Equal(t, "127.0.0.3", out.SerfConfig.MemberlistConfig.BindAddr)
	require.Equal(t, "127.0.0.3", conf.Addresses.HTTP)
	require.Equal(t, "127.0.0.3", conf.Addresses.RPC)
	require.Equal(t, "127.0.0.3", conf.Addresses.Serf)
	require.Equal(t, "127.0.0.3:4646", conf.normalizedAddrs.HTTP)
	require.Equal(t, "127.0.0.3:4647", conf.normalizedAddrs.RPC)
	require.Equal(t, "127.0.0.3:4648", conf.normalizedAddrs.Serf)

	// Properly handles the bootstrap flags
	conf.Server.BootstrapExpect = 1
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 1, out.BootstrapExpect)

	conf.Server.BootstrapExpect = 3
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 3, out.BootstrapExpect)
}

func TestAgent_ServerConfig_SchedulerFlags(t *testing.T) {
	cases := []struct {
		name     string
		input    *structs.SchedulerConfiguration
		expected structs.SchedulerConfiguration
	}{
		{
			"default case",
			nil,
			structs.SchedulerConfiguration{
				SchedulerAlgorithm: "binpack",
				PreemptionConfig: structs.PreemptionConfig{
					SystemSchedulerEnabled: true,
				},
			},
		},
		{
			"empty value: preemption is disabled",
			&structs.SchedulerConfiguration{},
			structs.SchedulerConfiguration{
				PreemptionConfig: structs.PreemptionConfig{
					SystemSchedulerEnabled: false,
				},
			},
		},
		{
			"all explicitly set",
			&structs.SchedulerConfiguration{
				PreemptionConfig: structs.PreemptionConfig{
					SystemSchedulerEnabled:  true,
					BatchSchedulerEnabled:   true,
					ServiceSchedulerEnabled: true,
				},
			},
			structs.SchedulerConfiguration{
				PreemptionConfig: structs.PreemptionConfig{
					SystemSchedulerEnabled:  true,
					BatchSchedulerEnabled:   true,
					ServiceSchedulerEnabled: true,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conf := DefaultConfig()
			conf.Server.DefaultSchedulerConfig = c.input

			a := &Agent{config: conf}
			conf.AdvertiseAddrs.Serf = "127.0.0.1:4000"
			conf.AdvertiseAddrs.RPC = "127.0.0.1:4001"
			conf.AdvertiseAddrs.HTTP = "10.10.11.1:4005"
			conf.ACL.Enabled = true
			require.NoError(t, conf.normalizeAddrs())

			out, err := a.serverConfig()
			require.NoError(t, err)
			require.Equal(t, c.expected, out.DefaultSchedulerConfig)
		})
	}
}

// TestAgent_ServerConfig_Limits_Errors asserts invalid Limits configurations
// cause errors. This is the server-only (RPC) counterpart to
// TestHTTPServer_Limits_Error.
func TestAgent_ServerConfig_Limits_Error(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		expectedErr string
		limits      sconfig.Limits
	}{
		{
			name:        "Negative Timeout",
			expectedErr: "rpc_handshake_timeout must be >= 0",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "-5s",
				RPCMaxConnsPerClient: helper.IntToPtr(100),
			},
		},
		{
			name:        "Invalid Timeout",
			expectedErr: "error parsing rpc_handshake_timeout",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "s",
				RPCMaxConnsPerClient: helper.IntToPtr(100),
			},
		},
		{
			name:        "Missing Timeout",
			expectedErr: "error parsing rpc_handshake_timeout",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "",
				RPCMaxConnsPerClient: helper.IntToPtr(100),
			},
		},
		{
			name:        "Negative Connection Limit",
			expectedErr: "rpc_max_conns_per_client must be > 25; found: -100",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "5s",
				RPCMaxConnsPerClient: helper.IntToPtr(-100),
			},
		},
		{
			name:        "Low Connection Limit",
			expectedErr: "rpc_max_conns_per_client must be > 25; found: 20",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "5s",
				RPCMaxConnsPerClient: helper.IntToPtr(sconfig.LimitsNonStreamingConnsPerClient),
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Limits = tc.limits
			serverConf, err := convertServerConfig(conf)
			assert.Nil(t, serverConf)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

// TestAgent_ServerConfig_Limits_OK asserts valid Limits configurations do not
// cause errors. This is the server-only (RPC) counterpart to
// TestHTTPServer_Limits_OK.
func TestAgent_ServerConfig_Limits_OK(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		limits sconfig.Limits
	}{
		{
			name:   "Default",
			limits: config.DefaultLimits(),
		},
		{
			name: "Zero+nil is valid to disable",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "0",
				RPCMaxConnsPerClient: nil,
			},
		},
		{
			name: "Zeros are valid",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "0s",
				RPCMaxConnsPerClient: helper.IntToPtr(0),
			},
		},
		{
			name: "Low limits are valid",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "1ms",
				RPCMaxConnsPerClient: helper.IntToPtr(26),
			},
		},
		{
			name: "High limits are valid",
			limits: sconfig.Limits{
				RPCHandshakeTimeout:  "5h",
				RPCMaxConnsPerClient: helper.IntToPtr(100000),
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Limits = tc.limits
			serverConf, err := convertServerConfig(conf)
			assert.NoError(t, err)
			require.NotNil(t, serverConf)
		})
	}
}

func TestAgent_ServerConfig_RaftMultiplier_Ok(t *testing.T) {
	t.Parallel()

	cases := []struct {
		multiplier         *int
		electionTimout     time.Duration
		heartbeatTimeout   time.Duration
		leaderLeaseTimeout time.Duration
		commitTimeout      time.Duration
	}{
		// nil, 0 are the defaults of the Raft library.
		// Expected values are hardcoded to detect changes from raft.
		{
			multiplier: nil,

			electionTimout:     1 * time.Second,
			heartbeatTimeout:   1 * time.Second,
			leaderLeaseTimeout: 500 * time.Millisecond,
			commitTimeout:      50 * time.Millisecond,
		},

		{
			multiplier: helper.IntToPtr(0),

			electionTimout:     1 * time.Second,
			heartbeatTimeout:   1 * time.Second,
			leaderLeaseTimeout: 500 * time.Millisecond,
			commitTimeout:      50 * time.Millisecond,
		},
		{
			multiplier: helper.IntToPtr(1),

			electionTimout:     1 * time.Second,
			heartbeatTimeout:   1 * time.Second,
			leaderLeaseTimeout: 500 * time.Millisecond,
			commitTimeout:      50 * time.Millisecond,
		},
		{
			multiplier: helper.IntToPtr(5),

			electionTimout:     5 * time.Second,
			heartbeatTimeout:   5 * time.Second,
			leaderLeaseTimeout: 2500 * time.Millisecond,
			commitTimeout:      250 * time.Millisecond,
		},
		{
			multiplier: helper.IntToPtr(6),

			electionTimout:     6 * time.Second,
			heartbeatTimeout:   6 * time.Second,
			leaderLeaseTimeout: 3000 * time.Millisecond,
			commitTimeout:      300 * time.Millisecond,
		},
		{
			multiplier: helper.IntToPtr(10),

			electionTimout:     10 * time.Second,
			heartbeatTimeout:   10 * time.Second,
			leaderLeaseTimeout: 5000 * time.Millisecond,
			commitTimeout:      500 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		v := "default"
		if tc.multiplier != nil {
			v = fmt.Sprintf("%v", *tc.multiplier)
		}
		t.Run(v, func(t *testing.T) {
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Server.RaftMultiplier = tc.multiplier

			serverConf, err := convertServerConfig(conf)
			require.NoError(t, err)

			assert.Equal(t, tc.electionTimout, serverConf.RaftConfig.ElectionTimeout, "election timeout")
			assert.Equal(t, tc.heartbeatTimeout, serverConf.RaftConfig.HeartbeatTimeout, "heartbeat timeout")
			assert.Equal(t, tc.leaderLeaseTimeout, serverConf.RaftConfig.LeaderLeaseTimeout, "leader lease timeout")
			assert.Equal(t, tc.commitTimeout, serverConf.RaftConfig.CommitTimeout, "commit timeout")
		})
	}
}

func TestAgent_ServerConfig_RaftMultiplier_Bad(t *testing.T) {
	t.Parallel()

	cases := []int{
		-1,
		100,
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Server.RaftMultiplier = &tc

			_, err := convertServerConfig(conf)
			require.Error(t, err)
			require.Contains(t, err.Error(), "raft_multiplier cannot be")
		})
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
func TestAgent_Client_TelemetryConfiguration(t *testing.T) {
	assert := assert.New(t)

	conf := DefaultConfig()
	conf.DevMode = true

	a := &Agent{config: conf}

	c, err := a.clientConfig()
	assert.Nil(err)

	telemetry := conf.Telemetry

	assert.Equal(c.StatsCollectionInterval, telemetry.collectionInterval)
	assert.Equal(c.PublishNodeMetrics, telemetry.PublishNodeMetrics)
	assert.Equal(c.PublishAllocationMetrics, telemetry.PublishAllocationMetrics)
}

// TestAgent_HTTPCheck asserts Agent.agentHTTPCheck properly alters the HTTP
// API health check depending on configuration.
func TestAgent_HTTPCheck(t *testing.T) {
	t.Parallel()
	logger := testlog.HCLogger(t)
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
		config: DevConfig(nil),
		logger: testlog.HCLogger(t),
	}
	if err := a.config.normalizeAddrs(); err != nil {
		t.Fatalf("error normalizing config: %v", err)
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

// Here we validate that log levels get updated when the configuration is
// reloaded. I can't find a good way to fetch this from the logger itself, so
// we pull it only from the agents configuration struct, not the logger.
func TestAgent_Reload_LogLevel(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.LogLevel = "INFO"
	})
	defer agent.Shutdown()

	assert.Equal("INFO", agent.GetConfig().LogLevel)

	newConfig := &Config{
		LogLevel: "TRACE",
	}

	assert.Nil(agent.Reload(newConfig))
	assert.Equal("TRACE", agent.GetConfig().LogLevel)
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
		auditor: &noOpAuditor{},
		config:  agentConfig,
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
		auditor: &noOpAuditor{},
		config:  agentConfig,
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

	logger := testlog.HCLogger(t)

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

	logger := testlog.HCLogger(t)

	agentConfig := &Config{
		TLSConfig: &sconfig.TLSConfig{},
	}

	agent := &Agent{
		auditor: &noOpAuditor{},
		logger:  logger,
		config:  agentConfig,
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

	logger := testlog.HCLogger(t)

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
		logger:  logger,
		config:  agentConfig,
		auditor: &noOpAuditor{},
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

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
	assert.False(shouldReloadAgent)
	assert.False(shouldReloadHTTP)
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

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
	require.True(shouldReloadAgent)
	require.True(shouldReloadHTTP)
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

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
	assert.True(shouldReloadAgent)
	assert.False(shouldReloadHTTP)
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

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(newConfig)
	assert.True(shouldReloadAgent)
	assert.True(shouldReloadHTTP)
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
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	tmpfn := filepath.Join(dir, "testcert")
	err = ioutil.WriteFile(tmpfn, content, 0666)
	require.Nil(err)

	const (
		cafile = "../../helper/tlsutil/testdata/ca.pem"
		key    = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	logger := testlog.HCLogger(t)

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

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(agentConfig)
	require.False(shouldReloadAgent)
	require.False(shouldReloadHTTP)

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

	shouldReloadAgent, shouldReloadHTTP = agent.ShouldReload(newAgentConfig)
	require.True(shouldReloadAgent)
	require.True(shouldReloadHTTP)
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
		shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
		require.True(shouldReloadAgent)
		require.True(shouldReloadHTTP)
	}

	err := agent.Reload(sameAgentConfig)
	require.Nil(err)

	{
		shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
		require.False(shouldReloadAgent)
		require.False(shouldReloadHTTP)
	}
}

func TestAgent_ProxyRPC_Dev(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	id := agent.client.NodeID()
	req := &structs.NodeSpecificRequest{
		NodeID: id,
		QueryOptions: structs.QueryOptions{
			Region: agent.server.Region(),
		},
	}

	time.Sleep(100 * time.Millisecond)

	var resp cstructs.ClientStatsResponse
	if err := agent.RPC("ClientStats.Stats", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
}
