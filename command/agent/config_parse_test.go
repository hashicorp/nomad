package agent

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestConfig_Parse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		File   string
		Result *Config
		Err    bool
	}{
		{
			"basic.hcl",
			&Config{
				Region:      "foobar",
				Datacenter:  "dc2",
				NodeName:    "my-web",
				DataDir:     "/tmp/nomad",
				LogLevel:    "ERR",
				BindAddr:    "192.168.0.1",
				EnableDebug: true,
				Ports: &Ports{
					HTTP: 1234,
					RPC:  2345,
					Serf: 3456,
				},
				Addresses: &Addresses{
					HTTP: "127.0.0.1",
					RPC:  "127.0.0.2",
					Serf: "127.0.0.3",
				},
				AdvertiseAddrs: &AdvertiseAddrs{
					RPC:  "127.0.0.3",
					Serf: "127.0.0.4",
				},
				Client: &ClientConfig{
					Enabled:   true,
					StateDir:  "/tmp/client-state",
					AllocDir:  "/tmp/alloc",
					Servers:   []string{"a.b.c:80", "127.0.0.1:1234"},
					NodeClass: "linux-medium-64bit",
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"1.1.1.1", "2.2.2.2"},
						RetryInterval:    time.Duration(15) * time.Second,
						RetryMaxAttempts: 3,
					},
					Meta: map[string]string{
						"foo": "bar",
						"baz": "zip",
					},
					Options: map[string]string{
						"foo": "bar",
						"baz": "zip",
					},
					ChrootEnv: map[string]string{
						"/opt/myapp/etc": "/etc",
						"/opt/myapp/bin": "/bin",
					},
					NetworkInterface: "eth0",
					NetworkSpeed:     100,
					CpuCompute:       4444,
					MemoryMB:         0,
					MaxKillTimeout:   "10s",
					ClientMinPort:    1000,
					ClientMaxPort:    2000,
					Reserved: &Resources{
						CPU:                 10,
						MemoryMB:            10,
						DiskMB:              10,
						IOPS:                10,
						ReservedPorts:       "1,100,10-12",
						ParsedReservedPorts: []int{1, 10, 11, 12, 100},
					},
					GCInterval:            6 * time.Second,
					GCParallelDestroys:    6,
					GCDiskUsageThreshold:  82,
					GCInodeUsageThreshold: 91,
					GCMaxAllocs:           50,
					NoHostUUID:            helper.BoolToPtr(false),
				},
				Server: &ServerConfig{
					Enabled:                true,
					AuthoritativeRegion:    "foobar",
					BootstrapExpect:        5,
					DataDir:                "/tmp/data",
					ProtocolVersion:        3,
					RaftProtocol:           3,
					NumSchedulers:          helper.IntToPtr(2),
					EnabledSchedulers:      []string{"test"},
					NodeGCThreshold:        "12h",
					EvalGCThreshold:        "12h",
					JobGCThreshold:         "12h",
					DeploymentGCThreshold:  "12h",
					HeartbeatGrace:         30 * time.Second,
					MinHeartbeatTTL:        33 * time.Second,
					MaxHeartbeatsPerSecond: 11.0,
					RetryJoin:              []string{"1.1.1.1", "2.2.2.2"},
					StartJoin:              []string{"1.1.1.1", "2.2.2.2"},
					RetryInterval:          15 * time.Second,
					RejoinAfterLeave:       true,
					RetryMaxAttempts:       3,
					NonVotingServer:        true,
					RedundancyZone:         "foo",
					UpgradeVersion:         "0.8.0",
					EncryptKey:             "abc",
					ServerJoin: &ServerJoin{
						RetryJoin:        []string{"1.1.1.1", "2.2.2.2"},
						RetryInterval:    time.Duration(15) * time.Second,
						RetryMaxAttempts: 3,
					},
				},
				ACL: &ACLConfig{
					Enabled:          true,
					TokenTTL:         60 * time.Second,
					PolicyTTL:        60 * time.Second,
					ReplicationToken: "foobar",
				},
				Telemetry: &Telemetry{
					StatsiteAddr:               "127.0.0.1:1234",
					StatsdAddr:                 "127.0.0.1:2345",
					PrometheusMetrics:          true,
					DisableHostname:            true,
					UseNodeName:                false,
					CollectionInterval:         "3s",
					collectionInterval:         3 * time.Second,
					PublishAllocationMetrics:   true,
					PublishNodeMetrics:         true,
					DisableTaggedMetrics:       true,
					BackwardsCompatibleMetrics: true,
				},
				LeaveOnInt:                true,
				LeaveOnTerm:               true,
				EnableSyslog:              true,
				SyslogFacility:            "LOCAL1",
				DisableUpdateCheck:        helper.BoolToPtr(true),
				DisableAnonymousSignature: true,
				Consul: &config.ConsulConfig{
					ServerServiceName:   "nomad",
					ServerHTTPCheckName: "nomad-server-http-health-check",
					ServerSerfCheckName: "nomad-server-serf-health-check",
					ServerRPCCheckName:  "nomad-server-rpc-health-check",
					ClientServiceName:   "nomad-client",
					ClientHTTPCheckName: "nomad-client-http-health-check",
					Addr:                "127.0.0.1:9500",
					Token:               "token1",
					Auth:                "username:pass",
					EnableSSL:           &trueValue,
					VerifySSL:           &trueValue,
					CAFile:              "/path/to/ca/file",
					CertFile:            "/path/to/cert/file",
					KeyFile:             "/path/to/key/file",
					ServerAutoJoin:      &trueValue,
					ClientAutoJoin:      &trueValue,
					AutoAdvertise:       &trueValue,
					ChecksUseAdvertise:  &trueValue,
				},
				Vault: &config.VaultConfig{
					Addr:                 "127.0.0.1:9500",
					AllowUnauthenticated: &trueValue,
					Enabled:              &falseValue,
					Role:                 "test_role",
					TLSCaFile:            "/path/to/ca/file",
					TLSCaPath:            "/path/to/ca",
					TLSCertFile:          "/path/to/cert/file",
					TLSKeyFile:           "/path/to/key/file",
					TLSServerName:        "foobar",
					TLSSkipVerify:        &trueValue,
					TaskTokenTTL:         "1s",
					Token:                "12345",
				},
				TLSConfig: &config.TLSConfig{
					EnableHTTP:                  true,
					EnableRPC:                   true,
					VerifyServerHostname:        true,
					CAFile:                      "foo",
					CertFile:                    "bar",
					KeyFile:                     "pipe",
					RPCUpgradeMode:              true,
					VerifyHTTPSClient:           true,
					TLSPreferServerCipherSuites: true,
					TLSCipherSuites:             "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					TLSMinVersion:               "tls12",
				},
				HTTPAPIResponseHeaders: map[string]string{
					"Access-Control-Allow-Origin": "*",
				},
				Sentinel: &config.SentinelConfig{
					Imports: []*config.SentinelImport{
						{
							Name: "foo",
							Path: "foo",
							Args: []string{"a", "b", "c"},
						},
						{
							Name: "bar",
							Path: "bar",
							Args: []string{"x", "y", "z"},
						},
					},
				},
				Autopilot: &config.AutopilotConfig{
					CleanupDeadServers:      &trueValue,
					ServerStabilizationTime: 23057 * time.Second,
					LastContactThreshold:    12705 * time.Second,
					MaxTrailingLogs:         17849,
					EnableRedundancyZones:   &trueValue,
					DisableUpgradeMigration: &trueValue,
					EnableCustomUpgrades:    &trueValue,
				},
			},
			false,
		},
		{
			"non-optional.hcl",
			&Config{
				Region:         "",
				Datacenter:     "",
				NodeName:       "",
				DataDir:        "",
				LogLevel:       "",
				BindAddr:       "",
				EnableDebug:    false,
				Ports:          nil,
				Addresses:      nil,
				AdvertiseAddrs: nil,
				Client: &ClientConfig{
					Enabled:               false,
					StateDir:              "",
					AllocDir:              "",
					Servers:               nil,
					NodeClass:             "",
					Meta:                  nil,
					Options:               nil,
					ChrootEnv:             nil,
					NetworkInterface:      "",
					NetworkSpeed:          0,
					CpuCompute:            0,
					MemoryMB:              5555,
					MaxKillTimeout:        "",
					ClientMinPort:         0,
					ClientMaxPort:         0,
					Reserved:              nil,
					GCInterval:            0,
					GCParallelDestroys:    0,
					GCDiskUsageThreshold:  0,
					GCInodeUsageThreshold: 0,
					GCMaxAllocs:           0,
					NoHostUUID:            nil,
				},
				Server:                    nil,
				ACL:                       nil,
				Telemetry:                 nil,
				LeaveOnInt:                false,
				LeaveOnTerm:               false,
				EnableSyslog:              false,
				SyslogFacility:            "",
				DisableUpdateCheck:        nil,
				DisableAnonymousSignature: false,
				Consul:                 nil,
				Vault:                  nil,
				TLSConfig:              nil,
				HTTPAPIResponseHeaders: nil,
				Sentinel:               nil,
			},
			false,
		},
	}

	for _, tc := range cases {
		require := require.New(t)
		t.Run(tc.File, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("./config-test-fixtures", tc.File))
			if err != nil {
				t.Fatalf("file: %s\n\n%s", tc.File, err)
			}

			actual, err := ParseConfigFile(path)
			if (err != nil) != tc.Err {
				t.Fatalf("file: %s\n\n%s", tc.File, err)
			}

			//panic(fmt.Sprintf("first: %+v \n second: %+v", actual.TLSConfig, tc.Result.TLSConfig))
			require.EqualValues(removeHelperAttributes(actual), tc.Result)
		})
	}
}

// In order to compare the Config struct after parsing, and from generating what
// is expected in the test, we need to remove helper attributes that are
// instantiated in the process of parsing the configuration
func removeHelperAttributes(c *Config) *Config {
	if c.TLSConfig != nil {
		c.TLSConfig.KeyLoader = nil
	}
	return c
}
