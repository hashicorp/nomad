// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgent_RPC_Ping(t *testing.T) {
	ci.Parallel(t)
	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	var out struct{}
	if err := agent.RPC("Status.Ping", &structs.GenericRequest{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ServerConfig(t *testing.T) {
	ci.Parallel(t)
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
	require.Equal(t, []string{"127.0.0.2:4646"}, conf.normalizedAddrs.HTTP)
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

	conf.Server.FailoverHeartbeatTTL = 337 * time.Second
	out, err = a.serverConfig()
	require.NoError(t, err)
	require.Equal(t, 337*time.Second, out.FailoverHeartbeatTTL)

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
	require.Equal(t, []string{"127.0.0.3:4646"}, conf.normalizedAddrs.HTTP)
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
	ci.Parallel(t)

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
	ci.Parallel(t)

	cases := []struct {
		name        string
		expectedErr string
		limits      config.Limits
	}{
		{
			name:        "Negative Timeout",
			expectedErr: "rpc_handshake_timeout must be >= 0",
			limits: config.Limits{
				RPCHandshakeTimeout:  "-5s",
				RPCMaxConnsPerClient: pointer.Of(100),
			},
		},
		{
			name:        "Invalid Timeout",
			expectedErr: "error parsing rpc_handshake_timeout",
			limits: config.Limits{
				RPCHandshakeTimeout:  "s",
				RPCMaxConnsPerClient: pointer.Of(100),
			},
		},
		{
			name:        "Missing Timeout",
			expectedErr: "error parsing rpc_handshake_timeout",
			limits: config.Limits{
				RPCHandshakeTimeout:  "",
				RPCMaxConnsPerClient: pointer.Of(100),
			},
		},
		{
			name:        "Negative Connection Limit",
			expectedErr: "rpc_max_conns_per_client must be > 25; found: -100",
			limits: config.Limits{
				RPCHandshakeTimeout:  "5s",
				RPCMaxConnsPerClient: pointer.Of(-100),
			},
		},
		{
			name:        "Low Connection Limit",
			expectedErr: "rpc_max_conns_per_client must be > 25; found: 20",
			limits: config.Limits{
				RPCHandshakeTimeout:  "5s",
				RPCMaxConnsPerClient: pointer.Of(config.LimitsNonStreamingConnsPerClient),
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
	ci.Parallel(t)

	cases := []struct {
		name   string
		limits config.Limits
	}{
		{
			name:   "Default",
			limits: config.DefaultLimits(),
		},
		{
			name: "Zero+nil is valid to disable",
			limits: config.Limits{
				RPCHandshakeTimeout:  "0",
				RPCMaxConnsPerClient: nil,
			},
		},
		{
			name: "Zeros are valid",
			limits: config.Limits{
				RPCHandshakeTimeout:  "0s",
				RPCMaxConnsPerClient: pointer.Of(0),
			},
		},
		{
			name: "Low limits are valid",
			limits: config.Limits{
				RPCHandshakeTimeout:  "1ms",
				RPCMaxConnsPerClient: pointer.Of(26),
			},
		},
		{
			name: "High limits are valid",
			limits: config.Limits{
				RPCHandshakeTimeout:  "5h",
				RPCMaxConnsPerClient: pointer.Of(100000),
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

func TestAgent_ServerConfig_PlanRejectionTracker(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name           string
		trackerConfig  *PlanRejectionTracker
		expectedConfig *PlanRejectionTracker
		expectedErr    string
	}{
		{
			name:          "default",
			trackerConfig: nil,
			expectedConfig: &PlanRejectionTracker{
				NodeThreshold: 100,
				NodeWindow:    5 * time.Minute,
			},
			expectedErr: "",
		},
		{
			name: "valid config",
			trackerConfig: &PlanRejectionTracker{
				Enabled:       pointer.Of(true),
				NodeThreshold: 123,
				NodeWindow:    17 * time.Minute,
			},
			expectedConfig: &PlanRejectionTracker{
				Enabled:       pointer.Of(true),
				NodeThreshold: 123,
				NodeWindow:    17 * time.Minute,
			},
			expectedErr: "",
		},
		{
			name: "invalid node window",
			trackerConfig: &PlanRejectionTracker{
				NodeThreshold: 123,
			},
			expectedErr: "node_window must be greater than 0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := DevConfig(nil)
			require.NoError(t, config.normalizeAddrs())

			if tc.trackerConfig != nil {
				config.Server.PlanRejectionTracker = tc.trackerConfig
			}

			serverConfig, err := convertServerConfig(config)

			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				if tc.expectedConfig.Enabled != nil {
					require.Equal(t,
						*tc.expectedConfig.Enabled,
						serverConfig.NodePlanRejectionEnabled,
					)
				}
				require.Equal(t,
					tc.expectedConfig.NodeThreshold,
					serverConfig.NodePlanRejectionThreshold,
				)
				require.Equal(t,
					tc.expectedConfig.NodeWindow,
					serverConfig.NodePlanRejectionWindow,
				)
			}
		})
	}
}

func TestAgent_ServerConfig_RaftMultiplier_Ok(t *testing.T) {
	ci.Parallel(t)

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
			multiplier: pointer.Of(0),

			electionTimout:     1 * time.Second,
			heartbeatTimeout:   1 * time.Second,
			leaderLeaseTimeout: 500 * time.Millisecond,
			commitTimeout:      50 * time.Millisecond,
		},
		{
			multiplier: pointer.Of(1),

			electionTimout:     1 * time.Second,
			heartbeatTimeout:   1 * time.Second,
			leaderLeaseTimeout: 500 * time.Millisecond,
			commitTimeout:      50 * time.Millisecond,
		},
		{
			multiplier: pointer.Of(5),

			electionTimout:     5 * time.Second,
			heartbeatTimeout:   5 * time.Second,
			leaderLeaseTimeout: 2500 * time.Millisecond,
			commitTimeout:      250 * time.Millisecond,
		},
		{
			multiplier: pointer.Of(6),

			electionTimout:     6 * time.Second,
			heartbeatTimeout:   6 * time.Second,
			leaderLeaseTimeout: 3000 * time.Millisecond,
			commitTimeout:      300 * time.Millisecond,
		},
		{
			multiplier: pointer.Of(10),

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
	ci.Parallel(t)

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

func TestAgent_ServerConfig_RaftTrailingLogs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		value  *int
		expect interface{}
		isErr  bool
	}{
		{
			name:   "bad",
			value:  pointer.Of(int(-1)),
			isErr:  true,
			expect: "raft_trailing_logs must be non-negative",
		},
		{
			name:   "good",
			value:  pointer.Of(int(10)),
			expect: uint64(10),
		},
		{
			name:   "empty",
			value:  nil,
			expect: raft.DefaultConfig().TrailingLogs,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc := tc
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Server.RaftTrailingLogs = tc.value
			nc, err := convertServerConfig(conf)

			if !tc.isErr {
				must.NoError(t, err)
				val := tc.expect.(uint64)
				must.Eq(t, val, nc.RaftConfig.TrailingLogs)
				return
			}
			must.Error(t, err)
			must.StrContains(t, err.Error(), tc.expect.(string))
		})
	}
}

func TestAgent_ServerConfig_RaftSnapshotThreshold(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		value  *int
		expect interface{}
		isErr  bool
	}{
		{
			name:   "bad",
			value:  pointer.Of(int(-1)),
			isErr:  true,
			expect: "raft_snapshot_threshold must be non-negative",
		},
		{
			name:   "good",
			value:  pointer.Of(int(10)),
			expect: uint64(10),
		},
		{
			name:   "empty",
			value:  nil,
			expect: raft.DefaultConfig().SnapshotThreshold,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc := tc
			conf := DevConfig(nil)
			require.NoError(t, conf.normalizeAddrs())

			conf.Server.RaftSnapshotThreshold = tc.value
			nc, err := convertServerConfig(conf)

			if !tc.isErr {
				must.NoError(t, err)
				val := tc.expect.(uint64)
				must.Eq(t, val, nc.RaftConfig.SnapshotThreshold)
				return
			}
			must.Error(t, err)
			must.StrContains(t, err.Error(), tc.expect.(string))
		})
	}
}

func TestAgent_ServerConfig_RaftProtocol_3(t *testing.T) {
	ci.Parallel(t)

	cases := []int{
		0, 1, 2, 3, 4,
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("protocol_version %d", tc), func(t *testing.T) {
			conf := DevConfig(nil)
			conf.Server.RaftProtocol = tc
			must.NoError(t, conf.normalizeAddrs())
			_, err := convertServerConfig(conf)

			switch tc {
			case 0, 3: // 0 defers to default
				must.NoError(t, err)
			default:
				exp := fmt.Sprintf("raft_protocol must be 3 in Nomad v1.4 and later, got %d", tc)
				must.EqError(t, err, exp)
			}
		})
	}
}

func TestAgent_ClientConfig_discovery(t *testing.T) {
	ci.Parallel(t)
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

	// Test the default, and then custom setting of the client service
	// discovery boolean.
	require.True(t, c.NomadServiceDiscovery)
	conf.Client.NomadServiceDiscovery = pointer.Of(false)
	c, err = a.clientConfig()
	require.NoError(t, err)
	require.False(t, c.NomadServiceDiscovery)
}

func TestAgent_ClientConfig_JobMaxSourceSize(t *testing.T) {
	ci.Parallel(t)

	conf := DevConfig(nil)
	must.Eq(t, conf.Server.JobMaxSourceSize, pointer.Of("1M"))
	must.NoError(t, conf.normalizeAddrs())

	// config conversion ensures value is set
	conf.Server.JobMaxSourceSize = nil
	agent := &Agent{config: conf}
	serverConf, err := agent.serverConfig()
	must.NoError(t, err)
	must.Eq(t, 1e6, serverConf.JobMaxSourceSize)
}

// Clients should inherit telemetry configuration
func TestAgent_Client_TelemetryConfiguration(t *testing.T) {
	ci.Parallel(t)

	conf := DefaultConfig()
	conf.DevMode = true

	a := &Agent{config: conf}

	c, err := a.clientConfig()
	must.NoError(t, err)

	telemetry := conf.Telemetry

	must.Eq(t, c.StatsCollectionInterval, telemetry.collectionInterval)
	must.Eq(t, c.PublishNodeMetrics, telemetry.PublishNodeMetrics)
	must.Eq(t, c.PublishAllocationMetrics, telemetry.PublishAllocationMetrics)
}

// TestAgent_HTTPCheck asserts Agent.agentHTTPCheck properly alters the HTTP
// API health check depending on configuration.
func TestAgent_HTTPCheck(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)
	agent := func() *Agent {
		return &Agent{
			logger: logger,
			config: &Config{
				AdvertiseAddrs:  &AdvertiseAddrs{HTTP: "advertise:4646"},
				normalizedAddrs: &NormalizedAddrs{HTTP: []string{"normalized:4646"}},
				Consul: &config.ConsulConfig{
					ChecksUseAdvertise: pointer.Of(false),
				},
				TLSConfig: &config.TLSConfig{EnableHTTP: false},
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
		if expected := a.config.normalizedAddrs.HTTP[0]; check.PortLabel != expected {
			t.Errorf("expected normalized addr not %q", check.PortLabel)
		}
	})

	t.Run("Plain HTTP + ChecksUseAdvertise", func(t *testing.T) {
		a := agent()
		a.config.Consul.ChecksUseAdvertise = pointer.Of(true)
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
	ci.Parallel(t)
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
	ci.Parallel(t)
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
	ci.Parallel(t)
	assert := assert.New(t)

	// We will start out with a bad cert and then reload with a good one.
	const (
		badca         = "../../helper/tlsutil/testdata/bad-agent-ca.pem"
		badcert       = "../../helper/tlsutil/testdata/badRegion-client-bad.pem"
		badkey        = "../../helper/tlsutil/testdata/badRegion-client-bad-key.pem"
		foocafile     = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooclientcert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fooclientkey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               badca,
			CertFile:             badcert,
			KeyFile:              badkey,
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
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               foocafile,
			CertFile:             fooclientcert,
			KeyFile:              fooclientkey,
		},
	}

	assert.Nil(agent.Reload(newConfig))
	assert.Equal(agent.Agent.config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.Equal(agent.Agent.config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
	assert.Equal(agent.Agent.config.TLSConfig.GetKeyLoader(), originalKeyloader)

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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		badca         = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		badcert       = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		badkey        = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
		cafile        = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		fooclientcert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fooclientkey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               badca,
			CertFile:             badcert,
			KeyFile:              badkey,
		},
	}

	agent := &Agent{
		auditor: &noOpAuditor{},
		config:  agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             fooclientcert,
			KeyFile:              fooclientkey,
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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		badca      = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		badcert    = "../../helper/tlsutil/testdata/badRegion-client-bad.pem"
		badkey     = "../../helper/tlsutil/testdata/badRegion-client-bad-key.pem"
		newfoocert = "invalid_cert_path"
		newfookey  = "invalid_key_path"
	)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               badca,
			CertFile:             badcert,
			KeyFile:              badkey,
		},
	}

	agent := &Agent{
		auditor: &noOpAuditor{},
		config:  agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               badca,
			CertFile:             newfoocert,
			KeyFile:              newfookey,
		},
	}

	err := agent.Reload(newConfig)
	assert.NotNil(err)
	assert.NotEqual(agent.config.TLSConfig.CertFile, newConfig.TLSConfig.CertFile)
	assert.NotEqual(agent.config.TLSConfig.KeyFile, newConfig.TLSConfig.KeyFile)
}

func Test_GetConfig(t *testing.T) {
	ci.Parallel(t)

	assert := assert.New(t)

	agentConfig := &Config{
		Telemetry:      &Telemetry{},
		Client:         &ClientConfig{},
		Server:         &ServerConfig{},
		ACL:            &ACLConfig{},
		Ports:          &Ports{},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		Vault:          &config.VaultConfig{},
		Consul:         &config.ConsulConfig{},
		Sentinel:       &config.SentinelConfig{},
	}

	agent := &Agent{
		config: agentConfig,
	}

	actualAgentConfig := agent.GetConfig()
	assert.Equal(actualAgentConfig, agentConfig)
}

func TestServer_Reload_TLS_WithNilConfiguration(t *testing.T) {
	ci.Parallel(t)
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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	logger := testlog.HCLogger(t)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{},
	}

	agent := &Agent{
		auditor: &noOpAuditor{},
		logger:  logger,
		config:  agentConfig,
	}

	newConfig := &Config{
		TLSConfig: &config.TLSConfig{
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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	logger := testlog.HCLogger(t)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{
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
		TLSConfig: &config.TLSConfig{},
	}

	assert.False(agentConfig.TLSConfig.IsEmpty())

	err := agent.Reload(newConfig)
	assert.Nil(err)

	assert.True(agent.config.TLSConfig.IsEmpty())
}

func TestServer_Reload_VaultConfig(t *testing.T) {
	ci.Parallel(t)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.Server.NumSchedulers = pointer.Of(0)
		c.Vault = &config.VaultConfig{
			Enabled:   pointer.Of(true),
			Token:     "vault-token",
			Namespace: "vault-namespace",
			Addr:      "https://vault.consul:8200",
		}
	})
	defer agent.Shutdown()

	newConfig := agent.GetConfig().Copy()
	newConfig.Vault = &config.VaultConfig{
		Enabled:   pointer.Of(true),
		Token:     "vault-token",
		Namespace: "another-namespace",
		Addr:      "https://vault.consul:8200",
	}

	sconf, err := convertServerConfig(newConfig)
	must.NoError(t, err)
	agent.finalizeServerConfig(sconf)

	// TODO: the vault client isn't accessible here, and we don't actually
	// overwrite the agent's server config on reload. We probably should? See
	// tests in nomad/server_test.go for verification of this code path's
	// behavior on the VaultClient
	must.NoError(t, agent.server.Reload(sconf))
}

func TestServer_ShouldReload_ReturnFalseForNoChanges(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	sameAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
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
	ci.Parallel(t)
	require := require.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	sameAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           false,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	sameAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
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
	ci.Parallel(t)
	assert := assert.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
		badcert = "../../helper/tlsutil/testdata/badRegion-client-bad.pem"
		badkey  = "../../helper/tlsutil/testdata/badRegion-client-bad-key.pem"
	)

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
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
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             badcert,
			KeyFile:              badkey,
		},
	}

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(newConfig)
	assert.True(shouldReloadAgent)
	assert.True(shouldReloadHTTP)
}

func TestServer_ShouldReload_ReturnTrueForFileChanges(t *testing.T) {
	ci.Parallel(t)
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
	dir := t.TempDir()

	tmpfn := filepath.Join(dir, "testcert")
	err := os.WriteFile(tmpfn, content, 0666)
	require.Nil(err)

	const (
		cafile = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		key    = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)

	logger := testlog.HCLogger(t)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{
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
	err = os.WriteFile(tmpfn, []byte(newCertificate), 0666)
	require.Nil(err)

	newAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
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
	ci.Parallel(t)
	require := require.New(t)

	const (
		cafile  = "../../helper/tlsutil/testdata/nomad-agent-ca.pem"
		foocert = "../../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		fookey  = "../../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
		badcert = "../../helper/tlsutil/testdata/badRegion-client-bad.pem"
		badkey  = "../../helper/tlsutil/testdata/badRegion-client-bad-key.pem"
	)

	sameAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             foocert,
			KeyFile:              fookey,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               cafile,
			CertFile:             badcert,
			KeyFile:              badkey,
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

func TestServer_ShouldReload_ReturnTrueForRPCUpgradeModeChanges(t *testing.T) {
	ci.Parallel(t)

	sameAgentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			RPCUpgradeMode: true,
		},
	}

	agent := NewTestAgent(t, t.Name(), func(c *Config) {
		c.TLSConfig = &config.TLSConfig{
			RPCUpgradeMode: false,
		}
	})
	defer agent.Shutdown()

	shouldReloadAgent, shouldReloadHTTP := agent.ShouldReload(sameAgentConfig)
	require.True(t, shouldReloadAgent)
	require.False(t, shouldReloadHTTP)
}

func TestAgent_ProxyRPC_Dev(t *testing.T) {
	ci.Parallel(t)
	agent := NewTestAgent(t, t.Name(), nil)
	defer agent.Shutdown()

	id := agent.client.NodeID()
	req := &structs.NodeSpecificRequest{
		NodeID: id,
		QueryOptions: structs.QueryOptions{
			Region: agent.server.Region(),
		},
	}

	testutil.WaitForResultUntil(time.Second,
		func() (bool, error) {
			var resp cstructs.ClientStatsResponse
			err := agent.RPC("ClientStats.Stats", req, &resp)
			if err != nil {
				return false, err
			}
			return true, nil
		},
		func(err error) {
			t.Fatalf("was unable to read ClientStats.Stats RPC: %v", err)
		})

}

func TestAgent_ServerConfig_JobMaxPriority_Ok(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		maxPriority    *int
		jobMaxPriority int
	}{
		{
			maxPriority:    nil,
			jobMaxPriority: 100,
		},

		{
			maxPriority:    pointer.Of(0),
			jobMaxPriority: 100,
		},
		{
			maxPriority:    pointer.Of(100),
			jobMaxPriority: 100,
		},
		{
			maxPriority:    pointer.Of(200),
			jobMaxPriority: 200,
		},
		{
			maxPriority:    pointer.Of(32766),
			jobMaxPriority: 32766,
		},
	}

	for _, tc := range cases {
		v := "default"
		if tc.maxPriority != nil {
			v = fmt.Sprintf("%v", *tc.maxPriority)
		}
		t.Run(v, func(t *testing.T) {
			conf := DevConfig(nil)
			must.NoError(t, conf.normalizeAddrs())

			conf.Server.JobMaxPriority = tc.maxPriority

			serverConf, err := convertServerConfig(conf)
			must.NoError(t, err)
			must.Eq(t, tc.jobMaxPriority, serverConf.JobMaxPriority)
		})
	}
}

func TestAgent_ServerConfig_JobMaxPriority_Bad(t *testing.T) {
	ci.Parallel(t)

	cases := []int{
		99,
		math.MaxInt16,
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			conf := DevConfig(nil)
			must.NoError(t, conf.normalizeAddrs())

			conf.Server.JobMaxPriority = &tc

			_, err := convertServerConfig(conf)
			must.Error(t, err)
			must.ErrorContains(t, err, "job_max_priority cannot be")
		})
	}
}

func TestAgent_ServerConfig_JobDefaultPriority_Ok(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		defaultPriority    *int
		jobDefaultPriority int
	}{
		{
			defaultPriority:    nil,
			jobDefaultPriority: 50,
		},

		{
			defaultPriority:    pointer.Of(0),
			jobDefaultPriority: 50,
		},
		{
			defaultPriority:    pointer.Of(50),
			jobDefaultPriority: 50,
		},
		{
			defaultPriority:    pointer.Of(60),
			jobDefaultPriority: 60,
		},
		{
			defaultPriority:    pointer.Of(99),
			jobDefaultPriority: 99,
		},
	}

	for _, tc := range cases {
		v := "default"
		if tc.defaultPriority != nil {
			v = fmt.Sprintf("%v", *tc.defaultPriority)
		}
		t.Run(v, func(t *testing.T) {
			conf := DevConfig(nil)
			must.NoError(t, conf.normalizeAddrs())

			conf.Server.JobDefaultPriority = tc.defaultPriority

			serverConf, err := convertServerConfig(conf)
			must.NoError(t, err)

			must.Eq(t, tc.jobDefaultPriority, serverConf.JobDefaultPriority)
		})
	}
}

func TestAgent_ServerConfig_JobDefaultPriority_Bad(t *testing.T) {
	ci.Parallel(t)

	cases := []int{
		49,
		100,
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			conf := DevConfig(nil)
			must.NoError(t, conf.normalizeAddrs())

			conf.Server.JobDefaultPriority = &tc

			_, err := convertServerConfig(conf)
			must.Error(t, err)
			must.ErrorContains(t, err, "job_default_priority cannot be")
		})
	}
}
