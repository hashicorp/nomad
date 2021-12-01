package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

var basicConfig = &Config{
	Region:      "foobar",
	Datacenter:  "dc2",
	NodeName:    "my-web",
	DataDir:     "/tmp/nomad",
	PluginDir:   "/tmp/nomad-plugins",
	LogFile:     "/var/log/nomad.log",
	LogLevel:    "ERR",
	LogJson:     true,
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
			RetryIntervalHCL: "15s",
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
			CPU:           10,
			MemoryMB:      10,
			DiskMB:        10,
			ReservedPorts: "1,100,10-12",
		},
		GCInterval:            6 * time.Second,
		GCIntervalHCL:         "6s",
		GCParallelDestroys:    6,
		GCDiskUsageThreshold:  82,
		GCInodeUsageThreshold: 91,
		GCMaxAllocs:           50,
		NoHostUUID:            helper.BoolToPtr(false),
		DisableRemoteExec:     true,
		HostVolumes: []*structs.ClientHostVolumeConfig{
			{Name: "tmp", Path: "/tmp"},
		},
		CNIPath:             "/tmp/cni_path",
		BridgeNetworkName:   "custom_bridge_name",
		BridgeNetworkSubnet: "custom_bridge_subnet",
	},
	Server: &ServerConfig{
		Enabled:                   true,
		AuthoritativeRegion:       "foobar",
		BootstrapExpect:           5,
		DataDir:                   "/tmp/data",
		RaftProtocol:              3,
		RaftMultiplier:            helper.IntToPtr(4),
		NumSchedulers:             helper.IntToPtr(2),
		EnabledSchedulers:         []string{"test"},
		NodeGCThreshold:           "12h",
		EvalGCThreshold:           "12h",
		JobGCInterval:             "3m",
		JobGCThreshold:            "12h",
		DeploymentGCThreshold:     "12h",
		CSIVolumeClaimGCThreshold: "12h",
		CSIPluginGCThreshold:      "12h",
		HeartbeatGrace:            30 * time.Second,
		HeartbeatGraceHCL:         "30s",
		MinHeartbeatTTL:           33 * time.Second,
		MinHeartbeatTTLHCL:        "33s",
		MaxHeartbeatsPerSecond:    11.0,
		FailoverHeartbeatTTL:      330 * time.Second,
		FailoverHeartbeatTTLHCL:   "330s",
		RetryJoin:                 []string{"1.1.1.1", "2.2.2.2"},
		StartJoin:                 []string{"1.1.1.1", "2.2.2.2"},
		RetryInterval:             15 * time.Second,
		RetryIntervalHCL:          "15s",
		RejoinAfterLeave:          true,
		RetryMaxAttempts:          3,
		NonVotingServer:           true,
		RedundancyZone:            "foo",
		UpgradeVersion:            "0.8.0",
		EncryptKey:                "abc",
		EnableEventBroker:         helper.BoolToPtr(false),
		EventBufferSize:           helper.IntToPtr(200),
		ServerJoin: &ServerJoin{
			RetryJoin:        []string{"1.1.1.1", "2.2.2.2"},
			RetryInterval:    time.Duration(15) * time.Second,
			RetryIntervalHCL: "15s",
			RetryMaxAttempts: 3,
		},
		DefaultSchedulerConfig: &structs.SchedulerConfiguration{
			SchedulerAlgorithm: "spread",
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled:  true,
				BatchSchedulerEnabled:   true,
				ServiceSchedulerEnabled: true,
			},
		},
		LicensePath: "/tmp/nomad.hclic",
	},
	ACL: &ACLConfig{
		Enabled:          true,
		TokenTTL:         60 * time.Second,
		TokenTTLHCL:      "60s",
		PolicyTTL:        60 * time.Second,
		PolicyTTLHCL:     "60s",
		ReplicationToken: "foobar",
	},
	Audit: &config.AuditConfig{
		Enabled: helper.BoolToPtr(true),
		Sinks: []*config.AuditSink{
			{
				DeliveryGuarantee: "enforced",
				Name:              "file",
				Type:              "file",
				Format:            "json",
				Path:              "/opt/nomad/audit.log",
				RotateDuration:    24 * time.Hour,
				RotateDurationHCL: "24h",
				RotateBytes:       100,
				RotateMaxFiles:    10,
			},
		},
		Filters: []*config.AuditFilter{
			{
				Name:       "default",
				Type:       "HTTPEvent",
				Endpoints:  []string{"/v1/metrics"},
				Stages:     []string{"*"},
				Operations: []string{"*"},
			},
		},
	},
	Telemetry: &Telemetry{
		StatsiteAddr:             "127.0.0.1:1234",
		StatsdAddr:               "127.0.0.1:2345",
		PrometheusMetrics:        true,
		DisableHostname:          true,
		UseNodeName:              false,
		CollectionInterval:       "3s",
		collectionInterval:       3 * time.Second,
		PublishAllocationMetrics: true,
		PublishNodeMetrics:       true,
	},
	LeaveOnInt:                true,
	LeaveOnTerm:               true,
	EnableSyslog:              true,
	SyslogFacility:            "LOCAL1",
	DisableUpdateCheck:        helper.BoolToPtr(true),
	DisableAnonymousSignature: true,
	Consul: &config.ConsulConfig{
		ServerServiceName:    "nomad",
		ServerHTTPCheckName:  "nomad-server-http-health-check",
		ServerSerfCheckName:  "nomad-server-serf-health-check",
		ServerRPCCheckName:   "nomad-server-rpc-health-check",
		ClientServiceName:    "nomad-client",
		ClientHTTPCheckName:  "nomad-client-http-health-check",
		Addr:                 "127.0.0.1:9500",
		AllowUnauthenticated: &trueValue,
		Token:                "token1",
		Auth:                 "username:pass",
		EnableSSL:            &trueValue,
		VerifySSL:            &trueValue,
		CAFile:               "/path/to/ca/file",
		CertFile:             "/path/to/cert/file",
		KeyFile:              "/path/to/key/file",
		ServerAutoJoin:       &trueValue,
		ClientAutoJoin:       &trueValue,
		AutoAdvertise:        &trueValue,
		ChecksUseAdvertise:   &trueValue,
		Timeout:              5 * time.Second,
	},
	Vault: &config.VaultConfig{
		Addr:                 "127.0.0.1:9500",
		AllowUnauthenticated: &trueValue,
		ConnectionRetryIntv:  config.DefaultVaultConnectRetryIntv,
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
		CleanupDeadServers:         &trueValue,
		ServerStabilizationTime:    23057 * time.Second,
		ServerStabilizationTimeHCL: "23057s",
		LastContactThreshold:       12705 * time.Second,
		LastContactThresholdHCL:    "12705s",
		MaxTrailingLogs:            17849,
		MinQuorum:                  3,
		EnableRedundancyZones:      &trueValue,
		DisableUpgradeMigration:    &trueValue,
		EnableCustomUpgrades:       &trueValue,
	},
	Plugins: []*config.PluginConfig{
		{
			Name: "docker",
			Args: []string{"foo", "bar"},
			Config: map[string]interface{}{
				"foo": "bar",
				"nested": []map[string]interface{}{
					{
						"bam": 2,
					},
				},
			},
		},
		{
			Name: "exec",
			Config: map[string]interface{}{
				"foo": true,
			},
		},
	},
}

var pluginConfig = &Config{
	Region:         "",
	Datacenter:     "",
	NodeName:       "",
	DataDir:        "",
	PluginDir:      "",
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
	Consul:                    nil,
	Vault:                     nil,
	TLSConfig:                 nil,
	HTTPAPIResponseHeaders:    map[string]string{},
	Sentinel:                  nil,
	Plugins: []*config.PluginConfig{
		{
			Name: "docker",
			Config: map[string]interface{}{
				"allow_privileged": true,
			},
		},
		{
			Name: "raw_exec",
			Config: map[string]interface{}{
				"enabled": true,
			},
		},
	},
}

var nonoptConfig = &Config{
	Region:         "",
	Datacenter:     "",
	NodeName:       "",
	DataDir:        "",
	PluginDir:      "",
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
	Consul:                    nil,
	Vault:                     nil,
	TLSConfig:                 nil,
	HTTPAPIResponseHeaders:    map[string]string{},
	Sentinel:                  nil,
}

func TestConfig_ParseMerge(t *testing.T) {
	t.Parallel()

	path, err := filepath.Abs(filepath.Join(".", "testdata", "basic.hcl"))
	require.NoError(t, err)

	actual, err := ParseConfigFile(path)
	require.NoError(t, err)

	require.Equal(t, basicConfig.Client, actual.Client)

	oldDefault := &Config{
		Consul:    config.DefaultConsulConfig(),
		Vault:     config.DefaultVaultConfig(),
		Autopilot: config.DefaultAutopilotConfig(),
		Client:    &ClientConfig{},
		Server:    &ServerConfig{},
		Audit:     &config.AuditConfig{},
	}
	merged := oldDefault.Merge(actual)
	require.Equal(t, basicConfig.Client, merged.Client)

}

func TestConfig_Parse(t *testing.T) {
	t.Parallel()

	basicConfig.addDefaults()
	pluginConfig.addDefaults()
	nonoptConfig.addDefaults()

	cases := []struct {
		File   string
		Result *Config
		Err    bool
	}{
		{
			"basic.hcl",
			basicConfig,
			false,
		},
		{
			"basic.json",
			basicConfig,
			false,
		},
		{
			"plugin.hcl",
			pluginConfig,
			false,
		},
		{
			"plugin.json",
			pluginConfig,
			false,
		},
		{
			"non-optional.hcl",
			nonoptConfig,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			require := require.New(t)
			path, err := filepath.Abs(filepath.Join("./testdata", tc.File))
			require.NoError(err)

			actual, err := ParseConfigFile(path)
			require.NoError(err)

			// ParseConfig used to re-merge defaults for these three objects,
			// despite them already being merged in LoadConfig. The test structs
			// expect these defaults to be set, but not the DefaultConfig
			// defaults, which include additional settings
			oldDefault := &Config{
				Consul:    config.DefaultConsulConfig(),
				Vault:     config.DefaultVaultConfig(),
				Autopilot: config.DefaultAutopilotConfig(),
			}
			actual = oldDefault.Merge(actual)

			require.EqualValues(tc.Result, removeHelperAttributes(actual))
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

func (c *Config) addDefaults() {
	if c.Client == nil {
		c.Client = &ClientConfig{}
	}
	if c.Client.ServerJoin == nil {
		c.Client.ServerJoin = &ServerJoin{}
	}
	if c.ACL == nil {
		c.ACL = &ACLConfig{}
	}
	if c.Audit == nil {
		c.Audit = &config.AuditConfig{}
	}
	if c.Consul == nil {
		c.Consul = config.DefaultConsulConfig()
	}
	if c.Autopilot == nil {
		c.Autopilot = config.DefaultAutopilotConfig()
	}
	if c.Vault == nil {
		c.Vault = config.DefaultVaultConfig()
	}
	if c.Telemetry == nil {
		c.Telemetry = &Telemetry{}
	}
	if c.Server == nil {
		c.Server = &ServerConfig{}
	}
	if c.Server.ServerJoin == nil {
		c.Server.ServerJoin = &ServerJoin{}
	}
}

// Tests for a panic parsing json with an object of exactly
// length 1 described in
// https://github.com/hashicorp/nomad/issues/1290
func TestConfig_ParsePanic(t *testing.T) {
	c, err := ParseConfigFile("./testdata/obj-len-one.hcl")
	if err != nil {
		t.Fatalf("parse error: %s\n", err)
	}

	d, err := ParseConfigFile("./testdata/obj-len-one.json")
	if err != nil {
		t.Fatalf("parse error: %s\n", err)
	}

	require.EqualValues(t, c, d)
}

// Top level keys left by hcl when parsing slices in the config
// structure should not be unexpected
func TestConfig_ParseSliceExtra(t *testing.T) {
	c, err := ParseConfigFile("./testdata/config-slices.json")
	require.NoError(t, err)

	opt := map[string]string{"o0": "foo", "o1": "bar"}
	meta := map[string]string{"m0": "foo", "m1": "bar", "m2": "true", "m3": "1.2"}
	env := map[string]string{"e0": "baz"}
	srv := []string{"foo", "bar"}

	require.EqualValues(t, opt, c.Client.Options)
	require.EqualValues(t, meta, c.Client.Meta)
	require.EqualValues(t, env, c.Client.ChrootEnv)
	require.EqualValues(t, srv, c.Client.Servers)
	require.EqualValues(t, srv, c.Server.EnabledSchedulers)
	require.EqualValues(t, srv, c.Server.StartJoin)
	require.EqualValues(t, srv, c.Server.RetryJoin)

	// the alt format is also accepted by hcl as valid config data
	c, err = ParseConfigFile("./testdata/config-slices-alt.json")
	require.NoError(t, err)

	require.EqualValues(t, opt, c.Client.Options)
	require.EqualValues(t, meta, c.Client.Meta)
	require.EqualValues(t, env, c.Client.ChrootEnv)
	require.EqualValues(t, srv, c.Client.Servers)
	require.EqualValues(t, srv, c.Server.EnabledSchedulers)
	require.EqualValues(t, srv, c.Server.StartJoin)
	require.EqualValues(t, srv, c.Server.RetryJoin)

	// small files keep more extra keys than large ones
	_, err = ParseConfigFile("./testdata/obj-len-one-server.json")
	require.NoError(t, err)
}

var sample0 = &Config{
	Region:     "global",
	Datacenter: "dc1",
	DataDir:    "/opt/data/nomad/data",
	LogLevel:   "INFO",
	BindAddr:   "0.0.0.0",
	AdvertiseAddrs: &AdvertiseAddrs{
		HTTP: "host.example.com",
		RPC:  "host.example.com",
		Serf: "host.example.com",
	},
	Client: &ClientConfig{ServerJoin: &ServerJoin{}},
	Server: &ServerConfig{
		Enabled:         true,
		BootstrapExpect: 3,
		RetryJoin:       []string{"10.0.0.101", "10.0.0.102", "10.0.0.103"},
		EncryptKey:      "sHck3WL6cxuhuY7Mso9BHA==",
		ServerJoin:      &ServerJoin{},
	},
	ACL: &ACLConfig{
		Enabled: true,
	},
	Audit: &config.AuditConfig{
		Enabled: helper.BoolToPtr(true),
		Sinks: []*config.AuditSink{
			{
				DeliveryGuarantee: "enforced",
				Name:              "file",
				Type:              "file",
				Format:            "json",
				Path:              "/opt/nomad/audit.log",
				RotateDuration:    24 * time.Hour,
				RotateDurationHCL: "24h",
				RotateBytes:       100,
				RotateMaxFiles:    10,
			},
		},
		Filters: []*config.AuditFilter{
			{
				Name:       "default",
				Type:       "HTTPEvent",
				Endpoints:  []string{"/v1/metrics"},
				Stages:     []string{"*"},
				Operations: []string{"*"},
			},
		},
	},
	Telemetry: &Telemetry{
		PrometheusMetrics:        true,
		DisableHostname:          true,
		CollectionInterval:       "60s",
		collectionInterval:       60 * time.Second,
		PublishAllocationMetrics: true,
		PublishNodeMetrics:       true,
	},
	LeaveOnInt:     true,
	LeaveOnTerm:    true,
	EnableSyslog:   true,
	SyslogFacility: "LOCAL0",
	Consul: &config.ConsulConfig{
		Token:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ServerAutoJoin: helper.BoolToPtr(false),
		ClientAutoJoin: helper.BoolToPtr(false),
	},
	Vault: &config.VaultConfig{
		Enabled: helper.BoolToPtr(true),
		Role:    "nomad-cluster",
		Addr:    "http://host.example.com:8200",
	},
	TLSConfig: &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "/opt/data/nomad/certs/nomad-ca.pem",
		CertFile:             "/opt/data/nomad/certs/server.pem",
		KeyFile:              "/opt/data/nomad/certs/server-key.pem",
	},
	Autopilot: &config.AutopilotConfig{
		CleanupDeadServers: helper.BoolToPtr(true),
	},
}

func TestConfig_ParseSample0(t *testing.T) {
	c, err := ParseConfigFile("./testdata/sample0.json")
	require.NoError(t, err)
	require.EqualValues(t, sample0, c)
}

var sample1 = &Config{
	Region:     "global",
	Datacenter: "dc1",
	DataDir:    "/opt/data/nomad/data",
	LogLevel:   "INFO",
	BindAddr:   "0.0.0.0",
	AdvertiseAddrs: &AdvertiseAddrs{
		HTTP: "host.example.com",
		RPC:  "host.example.com",
		Serf: "host.example.com",
	},
	Client: &ClientConfig{ServerJoin: &ServerJoin{}},
	Server: &ServerConfig{
		Enabled:         true,
		BootstrapExpect: 3,
		RetryJoin:       []string{"10.0.0.101", "10.0.0.102", "10.0.0.103"},
		EncryptKey:      "sHck3WL6cxuhuY7Mso9BHA==",
		ServerJoin:      &ServerJoin{},
	},
	ACL: &ACLConfig{
		Enabled: true,
	},
	Audit: &config.AuditConfig{
		Enabled: helper.BoolToPtr(true),
		Sinks: []*config.AuditSink{
			{
				Name:              "file",
				Type:              "file",
				DeliveryGuarantee: "enforced",
				Format:            "json",
				Path:              "/opt/nomad/audit.log",
				RotateDuration:    24 * time.Hour,
				RotateDurationHCL: "24h",
				RotateBytes:       100,
				RotateMaxFiles:    10,
			},
		},
		Filters: []*config.AuditFilter{
			{
				Name:       "default",
				Type:       "HTTPEvent",
				Endpoints:  []string{"/v1/metrics"},
				Stages:     []string{"*"},
				Operations: []string{"*"},
			},
		},
	},
	Telemetry: &Telemetry{
		PrometheusMetrics:        true,
		DisableHostname:          true,
		CollectionInterval:       "60s",
		collectionInterval:       60 * time.Second,
		PublishAllocationMetrics: true,
		PublishNodeMetrics:       true,
	},
	LeaveOnInt:     true,
	LeaveOnTerm:    true,
	EnableSyslog:   true,
	SyslogFacility: "LOCAL0",
	Consul: &config.ConsulConfig{
		EnableSSL:      helper.BoolToPtr(true),
		Token:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ServerAutoJoin: helper.BoolToPtr(false),
		ClientAutoJoin: helper.BoolToPtr(false),
	},
	Vault: &config.VaultConfig{
		Enabled: helper.BoolToPtr(true),
		Role:    "nomad-cluster",
		Addr:    "http://host.example.com:8200",
	},
	TLSConfig: &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "/opt/data/nomad/certs/nomad-ca.pem",
		CertFile:             "/opt/data/nomad/certs/server.pem",
		KeyFile:              "/opt/data/nomad/certs/server-key.pem",
	},
	Autopilot: &config.AutopilotConfig{
		CleanupDeadServers: helper.BoolToPtr(true),
	},
}

func TestConfig_ParseDir(t *testing.T) {
	c, err := LoadConfig("./testdata/sample1")
	require.NoError(t, err)

	// LoadConfig Merges all the config files in testdata/sample1, which makes empty
	// maps & slices rather than nil, so set those
	require.Empty(t, c.Client.Options)
	c.Client.Options = nil
	require.Empty(t, c.Client.Meta)
	c.Client.Meta = nil
	require.Empty(t, c.Client.ChrootEnv)
	c.Client.ChrootEnv = nil
	require.Empty(t, c.Server.StartJoin)
	c.Server.StartJoin = nil
	require.Empty(t, c.HTTPAPIResponseHeaders)
	c.HTTPAPIResponseHeaders = nil

	// LoadDir lists the config files
	expectedFiles := []string{
		"testdata/sample1/sample0.json",
		"testdata/sample1/sample1.json",
		"testdata/sample1/sample2.hcl",
	}
	require.Equal(t, expectedFiles, c.Files)
	c.Files = nil

	require.EqualValues(t, sample1, c)
}

// TestConfig_ParseDir_Matches_IndividualParsing asserts
// that parsing a directory config is the equivalent of
// parsing individual files in any order
func TestConfig_ParseDir_Matches_IndividualParsing(t *testing.T) {
	dirConfig, err := LoadConfig("./testdata/sample1")
	require.NoError(t, err)

	dirConfig = DefaultConfig().Merge(dirConfig)

	files := []string{
		"testdata/sample1/sample0.json",
		"testdata/sample1/sample1.json",
		"testdata/sample1/sample2.hcl",
	}

	for _, perm := range permutations(files) {
		t.Run(fmt.Sprintf("permutation %v", perm), func(t *testing.T) {
			config := DefaultConfig()

			for _, f := range perm {
				fc, err := LoadConfig(f)
				require.NoError(t, err)

				config = config.Merge(fc)
			}

			// sort files to get stable view
			sort.Strings(config.Files)
			sort.Strings(dirConfig.Files)

			require.EqualValues(t, dirConfig, config)
		})
	}

}

// https://stackoverflow.com/a/30226442
func permutations(arr []string) [][]string {
	var helper func([]string, int)
	res := [][]string{}

	helper = func(arr []string, n int) {
		if n == 1 {
			tmp := make([]string, len(arr))
			copy(tmp, arr)
			res = append(res, tmp)
		} else {
			for i := 0; i < n; i++ {
				helper(arr, n-1)
				if n%2 == 1 {
					tmp := arr[i]
					arr[i] = arr[n-1]
					arr[n-1] = tmp
				} else {
					tmp := arr[0]
					arr[0] = arr[n-1]
					arr[n-1] = tmp
				}
			}
		}
	}
	helper(arr, len(arr))
	return res
}
