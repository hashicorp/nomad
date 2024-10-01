// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

var basicConfig = &Config{
	Region:             "foobar",
	Datacenter:         "dc2",
	NodeName:           "my-web",
	DataDir:            "/tmp/nomad",
	PluginDir:          "/tmp/nomad-plugins",
	LogFile:            "/var/log/nomad.log",
	LogLevel:           "ERR",
	LogIncludeLocation: true,
	LogJson:            true,
	BindAddr:           "192.168.0.1",
	EnableDebug:        true,
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
		Enabled:        true,
		StateDir:       "/tmp/client-state",
		AllocDir:       "/tmp/alloc",
		AllocMountsDir: "/tmp/mounts",
		Servers:        []string{"a.b.c:80", "127.0.0.1:1234"},
		NodeClass:      "linux-medium-64bit",
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
		NoHostUUID:            pointer.Of(false),
		DisableRemoteExec:     true,
		HostVolumes: []*structs.ClientHostVolumeConfig{
			{Name: "tmp", Path: "/tmp"},
		},
		CNIPath:                 "/tmp/cni_path",
		BridgeNetworkName:       "custom_bridge_name",
		BridgeNetworkSubnet:     "custom_bridge_subnet",
		BridgeNetworkSubnetIPv6: "custom_bridge_subnet_ipv6",
	},
	Server: &ServerConfig{
		Enabled:                   true,
		AuthoritativeRegion:       "foobar",
		BootstrapExpect:           5,
		DataDir:                   "/tmp/data",
		RaftProtocol:              3,
		RaftMultiplier:            pointer.Of(4),
		NumSchedulers:             pointer.Of(2),
		EnabledSchedulers:         []string{"test"},
		NodeGCThreshold:           "12h",
		EvalGCThreshold:           "12h",
		JobGCInterval:             "3m",
		JobGCThreshold:            "12h",
		DeploymentGCThreshold:     "12h",
		CSIVolumeClaimGCInterval:  "3m",
		CSIVolumeClaimGCThreshold: "12h",
		CSIPluginGCThreshold:      "12h",
		ACLTokenGCThreshold:       "12h",
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
		EnableEventBroker:         pointer.Of(false),
		EventBufferSize:           pointer.Of(200),
		PlanRejectionTracker: &PlanRejectionTracker{
			Enabled:       pointer.Of(true),
			NodeThreshold: 100,
			NodeWindow:    41 * time.Minute,
			NodeWindowHCL: "41m",
		},
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
		LicensePath:        "/tmp/nomad.hclic",
		JobDefaultPriority: pointer.Of(100),
		JobMaxPriority:     pointer.Of(200),
	},
	ACL: &ACLConfig{
		Enabled:                  true,
		TokenTTL:                 60 * time.Second,
		TokenTTLHCL:              "60s",
		PolicyTTL:                60 * time.Second,
		PolicyTTLHCL:             "60s",
		RoleTTLHCL:               "60s",
		RoleTTL:                  60 * time.Second,
		TokenMinExpirationTTLHCL: "1h",
		TokenMinExpirationTTL:    1 * time.Hour,
		TokenMaxExpirationTTLHCL: "100h",
		TokenMaxExpirationTTL:    100 * time.Hour,
		ReplicationToken:         "foobar",
	},
	Audit: &config.AuditConfig{
		Enabled: pointer.Of(true),
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
		StatsiteAddr:               "127.0.0.1:1234",
		StatsdAddr:                 "127.0.0.1:2345",
		PrometheusMetrics:          true,
		DisableHostname:            true,
		UseNodeName:                false,
		InMemoryCollectionInterval: "1m",
		inMemoryCollectionInterval: 1 * time.Minute,
		InMemoryRetentionPeriod:    "24h",
		inMemoryRetentionPeriod:    24 * time.Hour,
		CollectionInterval:         "3s",
		collectionInterval:         3 * time.Second,
		PublishAllocationMetrics:   true,
		PublishNodeMetrics:         true,
	},
	LeaveOnInt:                true,
	LeaveOnTerm:               true,
	EnableSyslog:              true,
	SyslogFacility:            "LOCAL1",
	DisableUpdateCheck:        pointer.Of(true),
	DisableAnonymousSignature: true,
	Consuls: []*config.ConsulConfig{{
		Name:                      structs.ConsulDefaultCluster,
		ServerServiceName:         "nomad",
		ServerHTTPCheckName:       "nomad-server-http-health-check",
		ServerSerfCheckName:       "nomad-server-serf-health-check",
		ServerRPCCheckName:        "nomad-server-rpc-health-check",
		ClientServiceName:         "nomad-client",
		ClientHTTPCheckName:       "nomad-client-http-health-check",
		Addr:                      "127.0.0.1:9500",
		AllowUnauthenticated:      &trueValue,
		Token:                     "token1",
		Auth:                      "username:pass",
		EnableSSL:                 &trueValue,
		VerifySSL:                 &trueValue,
		CAFile:                    "/path/to/ca/file",
		CertFile:                  "/path/to/cert/file",
		KeyFile:                   "/path/to/key/file",
		ServerAutoJoin:            &trueValue,
		ClientAutoJoin:            &trueValue,
		AutoAdvertise:             &trueValue,
		ChecksUseAdvertise:        &trueValue,
		Timeout:                   5 * time.Second,
		TimeoutHCL:                "5s",
		ServiceIdentityAuthMethod: "nomad-services",
		ServiceIdentity: &config.WorkloadIdentityConfig{
			Audience: []string{"consul.io", "nomad.dev"},
			Env:      pointer.Of(false),
			File:     pointer.Of(true),
			TTL:      pointer.Of(1 * time.Hour),
			TTLHCL:   "1h",
		},
		TaskIdentityAuthMethod: "nomad-tasks",
		TaskIdentity: &config.WorkloadIdentityConfig{
			Audience: []string{"consul.io"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
			TTL:      pointer.Of(2 * time.Hour),
			TTLHCL:   "2h",
		},
	}},
	Vaults: []*config.VaultConfig{{
		Name:                 structs.VaultDefaultCluster,
		Addr:                 "127.0.0.1:9500",
		JWTAuthBackendPath:   "nomad_jwt",
		ConnectionRetryIntv:  30 * time.Second,
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
		DefaultIdentity: &config.WorkloadIdentityConfig{
			Audience: []string{"vault.io", "nomad.io"},
			Env:      pointer.Of(false),
			File:     pointer.Of(true),
			TTL:      pointer.Of(3 * time.Hour),
			TTLHCL:   "3h",
		},
	}},
	TLSConfig: &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "foo",
		CertFile:             "bar",
		KeyFile:              "pipe",
		RPCUpgradeMode:       true,
		VerifyHTTPSClient:    true,
		TLSCipherSuites:      "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		TLSMinVersion:        "tls12",
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
	Reporting: &config.ReportingConfig{
		ExportAddress:     "http://localhost:8080",
		ExportIntervalHCL: "15m",
		ExportInterval:    time.Minute * 15,
		License: &config.LicenseReportingConfig{
			Enabled: pointer.Of(true),
		},
	},
	KEKProviders: []*structs.KEKProviderConfig{
		{
			Provider: "aead",
			Active:   false,
		},
		{
			Provider: "awskms",
			Active:   true,
			Config: map[string]string{
				"region":     "us-east-1",
				"kms_key_id": "alias/kms-nomad-keyring",
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
	Reporting: &config.ReportingConfig{
		License: &config.LicenseReportingConfig{},
	},
	Consuls: []*config.ConsulConfig{},
	Vaults:  []*config.VaultConfig{},
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
	TLSConfig:                 nil,
	HTTPAPIResponseHeaders:    map[string]string{},
	Sentinel:                  nil,
	Reporting: &config.ReportingConfig{
		License: &config.LicenseReportingConfig{},
	},
	Consuls: []*config.ConsulConfig{},
	Vaults:  []*config.VaultConfig{},
}

func TestConfig_ParseMerge(t *testing.T) {
	ci.Parallel(t)

	path, err := filepath.Abs(filepath.Join(".", "testdata", "basic.hcl"))
	must.NoError(t, err)

	actual, err := ParseConfigFile(path)
	must.NoError(t, err)

	// The Vault connection retry interval is an internal only configuration
	// option, and therefore needs to be added here to ensure the test passes.
	actual.Vaults[0].ConnectionRetryIntv = config.DefaultVaultConnectRetryIntv
	must.Eq(t, basicConfig, actual)

	oldDefault := &Config{
		Autopilot: config.DefaultAutopilotConfig(),
		Client:    &ClientConfig{},
		Server:    &ServerConfig{},
		Audit:     &config.AuditConfig{},
	}
	merged := oldDefault.Merge(actual)
	must.Eq(t, basicConfig, merged)
}

func TestConfig_Parse(t *testing.T) {
	ci.Parallel(t)

	basicConfig.addDefaults()
	pluginConfig.addDefaults()
	nonoptConfig.addDefaults()

	cases := []struct {
		File   string
		Result *Config
	}{
		{
			"basic.hcl",
			basicConfig,
		},
		{
			"basic.json",
			basicConfig,
		},
		{
			"plugin.hcl",
			pluginConfig,
		},
		{
			"plugin.json",
			pluginConfig,
		},
		{
			"non-optional.hcl",
			nonoptConfig,
		},
	}

	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {

			path, err := filepath.Abs(filepath.Join("./testdata", tc.File))
			must.NoError(t, err)

			actual, err := ParseConfigFile(path)
			must.NoError(t, err)

			// The test assertion structs expect these defaults to be set, but
			// not the DefaultConfig defaults, which include a large number of
			// additional settings.
			oldDefault := &Config{
				Autopilot: config.DefaultAutopilotConfig(),
				Reporting: config.DefaultReporting(),
			}
			actual = oldDefault.Merge(actual)

			must.Eq(t, tc.Result, removeHelperAttributes(actual))
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
	if c.Consuls == nil {
		c.Consuls = []*config.ConsulConfig{config.DefaultConsulConfig()}
	}
	if c.Autopilot == nil {
		c.Autopilot = config.DefaultAutopilotConfig()
	}
	if c.Vaults == nil {
		c.Vaults = []*config.VaultConfig{config.DefaultVaultConfig()}
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
	if c.Server.PlanRejectionTracker == nil {
		c.Server.PlanRejectionTracker = &PlanRejectionTracker{}
	}
	if c.Reporting == nil {
		c.Reporting = &config.ReportingConfig{
			License: &config.LicenseReportingConfig{
				Enabled: pointer.Of(false),
			},
		}
	}
}

// Tests for a panic parsing json with an object of exactly
// length 1 described in
// https://github.com/hashicorp/nomad/issues/1290
func TestConfig_ParsePanic(t *testing.T) {
	ci.Parallel(t)

	c, err := ParseConfigFile("./testdata/obj-len-one.hcl")
	if err != nil {
		t.Fatalf("parse error: %s\n", err)
	}

	d, err := ParseConfigFile("./testdata/obj-len-one.json")
	if err != nil {
		t.Fatalf("parse error: %s\n", err)
	}

	must.Eq(t, c, d)
}

// Top level keys left by hcl when parsing slices in the config
// structure should not be unexpected
func TestConfig_ParseSliceExtra(t *testing.T) {
	ci.Parallel(t)

	c, err := ParseConfigFile("./testdata/config-slices.json")
	must.NoError(t, err)

	opt := map[string]string{"o0": "foo", "o1": "bar"}
	meta := map[string]string{"m0": "foo", "m1": "bar", "m2": "true", "m3": "1.2"}
	env := map[string]string{"e0": "baz"}
	srv := []string{"foo", "bar"}

	must.Eq(t, opt, c.Client.Options)
	must.Eq(t, meta, c.Client.Meta)
	must.Eq(t, env, c.Client.ChrootEnv)
	must.Eq(t, srv, c.Client.Servers)
	must.Eq(t, srv, c.Server.EnabledSchedulers)
	must.Eq(t, srv, c.Server.StartJoin)
	must.Eq(t, srv, c.Server.RetryJoin)

	// the alt format is also accepted by hcl as valid config data
	c, err = ParseConfigFile("./testdata/config-slices-alt.json")
	must.NoError(t, err)

	must.Eq(t, opt, c.Client.Options)
	must.Eq(t, meta, c.Client.Meta)
	must.Eq(t, env, c.Client.ChrootEnv)
	must.Eq(t, srv, c.Client.Servers)
	must.Eq(t, srv, c.Server.EnabledSchedulers)
	must.Eq(t, srv, c.Server.StartJoin)
	must.Eq(t, srv, c.Server.RetryJoin)

	// small files keep more extra keys than large ones
	_, err = ParseConfigFile("./testdata/obj-len-one-server.json")
	must.NoError(t, err)
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
		PlanRejectionTracker: &PlanRejectionTracker{
			NodeThreshold: 100,
			NodeWindow:    31 * time.Minute,
			NodeWindowHCL: "31m",
		},
	},
	ACL: &ACLConfig{
		Enabled: true,
	},
	Audit: &config.AuditConfig{
		Enabled: pointer.Of(true),
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
	Consuls: []*config.ConsulConfig{{
		Name:           structs.ConsulDefaultCluster,
		Token:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ServerAutoJoin: pointer.Of(false),
		ClientAutoJoin: pointer.Of(false),
	}},
	Vaults: []*config.VaultConfig{{
		Name:    structs.VaultDefaultCluster,
		Enabled: pointer.Of(true),
		Role:    "nomad-cluster",
		Addr:    "http://host.example.com:8200",
	}},
	TLSConfig: &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "/opt/data/nomad/certs/nomad-ca.pem",
		CertFile:             "/opt/data/nomad/certs/server.pem",
		KeyFile:              "/opt/data/nomad/certs/server-key.pem",
	},
	Autopilot: &config.AutopilotConfig{
		CleanupDeadServers: pointer.Of(true),
	},
	Reporting: config.DefaultReporting(),
	KEKProviders: []*structs.KEKProviderConfig{
		{
			Provider: "awskms",
			Active:   true,
			Config: map[string]string{
				"region":     "us-east-1",
				"kms_key_id": "alias/kms-nomad-keyring",
			},
		},
	},
}

func TestConfig_ParseSample0(t *testing.T) {
	ci.Parallel(t)

	c, err := ParseConfigFile("./testdata/sample0.json")
	must.NoError(t, err)
	must.Eq(t, sample0, c)
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
		PlanRejectionTracker: &PlanRejectionTracker{
			NodeThreshold: 100,
			NodeWindow:    31 * time.Minute,
			NodeWindowHCL: "31m",
		},
	},
	ACL: &ACLConfig{
		Enabled: true,
	},
	Audit: &config.AuditConfig{
		Enabled: pointer.Of(true),
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
	Consuls: []*config.ConsulConfig{{
		Name:                      structs.ConsulDefaultCluster,
		EnableSSL:                 pointer.Of(true),
		Token:                     "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ServerAutoJoin:            pointer.Of(false),
		ClientAutoJoin:            pointer.Of(false),
		ServerServiceName:         "nomad",
		ServerHTTPCheckName:       "Nomad Server HTTP Check",
		ServerSerfCheckName:       "Nomad Server Serf Check",
		ServerRPCCheckName:        "Nomad Server RPC Check",
		ClientServiceName:         "nomad-client",
		ClientHTTPCheckName:       "Nomad Client HTTP Check",
		AutoAdvertise:             pointer.Of(true),
		ChecksUseAdvertise:        pointer.Of(false),
		AllowUnauthenticated:      pointer.Of(true),
		Timeout:                   5 * time.Second,
		ServiceIdentityAuthMethod: structs.ConsulWorkloadsDefaultAuthMethodName,
		TaskIdentityAuthMethod:    structs.ConsulWorkloadsDefaultAuthMethodName,
		Addr:                      "127.0.0.1:8500",
		VerifySSL:                 pointer.Of(true),
	}},
	Vaults: []*config.VaultConfig{{
		Name:                 structs.VaultDefaultCluster,
		Enabled:              pointer.Of(true),
		Role:                 "nomad-cluster",
		Addr:                 "http://host.example.com:8200",
		JWTAuthBackendPath:   "jwt-nomad",
		ConnectionRetryIntv:  30 * time.Second,
		AllowUnauthenticated: pointer.Of(true),
	}},
	TLSConfig: &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "/opt/data/nomad/certs/nomad-ca.pem",
		CertFile:             "/opt/data/nomad/certs/server.pem",
		KeyFile:              "/opt/data/nomad/certs/server-key.pem",
	},
	Autopilot: &config.AutopilotConfig{
		CleanupDeadServers: pointer.Of(true),
	},
	Reporting: &config.ReportingConfig{
		License: &config.LicenseReportingConfig{},
	},
	KEKProviders: []*structs.KEKProviderConfig{
		{
			Provider: "aead",
			Active:   false,
		},
		{
			Provider: "awskms",
			Active:   true,
			Config: map[string]string{
				"region":     "us-east-1",
				"kms_key_id": "alias/kms-nomad-keyring",
			},
		},
	},
}

func TestConfig_ParseDir(t *testing.T) {
	ci.Parallel(t)

	c, err := LoadConfig("./testdata/sample1")
	must.NoError(t, err)

	// LoadConfig Merges all the config files in testdata/sample1, which makes empty
	// maps & slices rather than nil, so set those
	must.Zero(t, len(c.Client.Options))
	c.Client.Options = nil
	must.Zero(t, len(c.Client.Meta))
	c.Client.Meta = nil
	must.Zero(t, len(c.Client.ChrootEnv))
	c.Client.ChrootEnv = nil
	must.Zero(t, len(c.Server.StartJoin))
	c.Server.StartJoin = nil
	must.Zero(t, len(c.HTTPAPIResponseHeaders))
	c.HTTPAPIResponseHeaders = nil

	// LoadDir lists the config files
	expectedFiles := []string{
		"testdata/sample1/sample0.json",
		"testdata/sample1/sample1.json",
		"testdata/sample1/sample2.hcl",
	}
	must.Eq(t, expectedFiles, c.Files)
	c.Files = nil

	must.Eq(t, sample1, c)
}

// TestConfig_ParseDir_Matches_IndividualParsing asserts
// that parsing a directory config is the equivalent of
// parsing individual files in any order
func TestConfig_ParseDir_Matches_IndividualParsing(t *testing.T) {
	ci.Parallel(t)

	dirConfig, err := LoadConfig("./testdata/sample1")
	must.NoError(t, err)

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
				must.NoError(t, err)

				config = config.Merge(fc)
			}

			// sort files to get stable view
			sort.Strings(config.Files)
			sort.Strings(dirConfig.Files)

			must.Eq(t, dirConfig, config)
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

func TestConfig_MultipleVault(t *testing.T) {

	for _, suffix := range []string{"hcl", "json"} {
		t.Run(suffix, func(t *testing.T) {

			// verify the default Vault config is set from the list
			cfg := DefaultConfig()
			must.Len(t, 1, cfg.Vaults)
			defaultVault := cfg.Vaults[0]
			must.Eq(t, structs.VaultDefaultCluster, defaultVault.Name)
			must.Equal(t, config.DefaultVaultConfig(), defaultVault)
			must.Nil(t, defaultVault.Enabled) // unset
			must.Eq(t, "https://vault.service.consul:8200", defaultVault.Addr)
			must.Eq(t, "", defaultVault.Token)
			must.Eq(t, "jwt-nomad", defaultVault.JWTAuthBackendPath)

			// merge in the user's configuration
			fc, err := LoadConfig("testdata/basic." + suffix)
			must.NoError(t, err)
			cfg = cfg.Merge(fc)

			must.Len(t, 1, cfg.Vaults)
			defaultVault = cfg.Vaults[0]
			must.Eq(t, structs.VaultDefaultCluster, defaultVault.Name)
			must.NotNil(t, defaultVault.Enabled, must.Sprint("override should set to non-nil"))
			must.False(t, *defaultVault.Enabled)
			must.Eq(t, "127.0.0.1:9500", defaultVault.Addr)
			must.Eq(t, "nomad_jwt", defaultVault.JWTAuthBackendPath)
			must.Eq(t, "12345", defaultVault.Token)

			// add an extra Vault config and override fields in the default
			fc, err = LoadConfig("testdata/extra-vault." + suffix)
			must.NoError(t, err)

			cfg = cfg.Merge(fc)

			must.Len(t, 3, cfg.Vaults)
			defaultVault = cfg.Vaults[0]
			must.Eq(t, structs.VaultDefaultCluster, defaultVault.Name)
			must.True(t, *defaultVault.Enabled)
			must.Eq(t, "127.0.0.1:9500", defaultVault.Addr)
			must.Eq(t, "abracadabra", defaultVault.Token)

			must.Eq(t, "alternate", cfg.Vaults[1].Name)
			must.True(t, *cfg.Vaults[1].Enabled)
			must.Eq(t, "127.0.0.1:9501", cfg.Vaults[1].Addr)
			must.Eq(t, "xyzzy", cfg.Vaults[1].Token)

			must.Eq(t, "other", cfg.Vaults[2].Name)
			must.Nil(t, cfg.Vaults[2].Enabled)
			must.Eq(t, "127.0.0.1:9502", cfg.Vaults[2].Addr)
			must.Eq(t, pointer.Of(4*time.Hour), cfg.Vaults[2].DefaultIdentity.TTL)

			// check that extra Vault clusters have the defaults applied when not
			// overridden
			must.Eq(t, "jwt-nomad", cfg.Vaults[2].JWTAuthBackendPath)
		})
	}
}

func TestConfig_MultipleConsul(t *testing.T) {

	for _, suffix := range []string{"hcl", "json"} {
		t.Run(suffix, func(t *testing.T) {
			// verify the default Consul config is set from the list
			cfg := DefaultConfig()

			must.Len(t, 1, cfg.Consuls)
			defaultConsul := cfg.Consuls[0]
			must.Eq(t, structs.ConsulDefaultCluster, defaultConsul.Name)
			must.Eq(t, config.DefaultConsulConfig(), defaultConsul)
			must.True(t, *defaultConsul.AllowUnauthenticated)
			must.Eq(t, "127.0.0.1:8500", defaultConsul.Addr)
			must.Eq(t, "", defaultConsul.Token)

			// merge in the user's configuration which overrides fields in the
			// default config
			fc, err := LoadConfig("testdata/basic." + suffix)
			must.NoError(t, err)
			cfg = cfg.Merge(fc)

			must.Len(t, 1, cfg.Consuls)
			defaultConsul = cfg.Consuls[0]
			must.Eq(t, structs.ConsulDefaultCluster, defaultConsul.Name)
			must.True(t, *defaultConsul.AllowUnauthenticated)
			must.Eq(t, "127.0.0.1:9500", defaultConsul.Addr)
			must.Eq(t, "token1", defaultConsul.Token)

			// add an extra Consul config and override fields in the default
			fc, err = LoadConfig("testdata/extra-consul." + suffix)
			must.NoError(t, err)
			cfg = cfg.Merge(fc)

			must.Len(t, 3, cfg.Consuls)
			defaultConsul = cfg.Consuls[0]
			must.Eq(t, structs.ConsulDefaultCluster, defaultConsul.Name)
			must.False(t, *defaultConsul.AllowUnauthenticated)
			must.Eq(t, "127.0.0.1:9501", defaultConsul.Addr)
			must.Eq(t, "abracadabra", defaultConsul.Token)

			must.Eq(t, "alternate", cfg.Consuls[1].Name)
			must.True(t, *cfg.Consuls[1].AllowUnauthenticated)
			must.Eq(t, "127.0.0.2:8501", cfg.Consuls[1].Addr)
			must.Eq(t, "xyzzy", cfg.Consuls[1].Token)

			must.Eq(t, "other", cfg.Consuls[2].Name)
			must.Eq(t, pointer.Of(3*time.Hour), cfg.Consuls[2].ServiceIdentity.TTL)
			must.Eq(t, pointer.Of(5*time.Hour), cfg.Consuls[2].TaskIdentity.TTL)

			// check that extra Consul clusters have the defaults applied when
			// not overridden
			must.Eq(t, "nomad-client", cfg.Consuls[2].ClientServiceName)
		})
	}
}

func TestConfig_Telemetry(t *testing.T) {
	ci.Parallel(t)

	// Ensure merging a mostly empty struct correctly inherits default values
	// set.
	inputTelemetry1 := &Telemetry{PrometheusMetrics: true}
	mergedTelemetry1 := DefaultConfig().Telemetry.Merge(inputTelemetry1)
	must.Eq(t, mergedTelemetry1.inMemoryCollectionInterval, 10*time.Second)
	must.Eq(t, mergedTelemetry1.inMemoryRetentionPeriod, 1*time.Minute)

	// Ensure we can then overlay user specified data.
	inputTelemetry2 := &Telemetry{
		inMemoryCollectionInterval: 1 * time.Second,
		inMemoryRetentionPeriod:    10 * time.Second,
	}
	mergedTelemetry2 := mergedTelemetry1.Merge(inputTelemetry2)
	must.Eq(t, mergedTelemetry2.inMemoryCollectionInterval, 1*time.Second)
	must.Eq(t, mergedTelemetry2.inMemoryRetentionPeriod, 10*time.Second)
}

func TestConfig_Template(t *testing.T) {
	ci.Parallel(t)

	for _, suffix := range []string{"hcl", "json"} {
		t.Run(suffix, func(t *testing.T) {
			cfg := DefaultConfig()
			fc, err := LoadConfig("testdata/template." + suffix)
			must.NoError(t, err)
			cfg = cfg.Merge(fc)

			must.Eq(t, []string{"plugin"}, cfg.Client.TemplateConfig.FunctionDenylist)
			must.True(t, cfg.Client.TemplateConfig.DisableSandbox)
			must.Eq(t, pointer.Of(7600*time.Hour), cfg.Client.TemplateConfig.MaxStale)
			must.Eq(t, pointer.Of(10*time.Minute), cfg.Client.TemplateConfig.BlockQueryWaitTime)

			must.NotNil(t, cfg.Client.TemplateConfig.Wait)
			must.Eq(t, pointer.Of(10*time.Second), cfg.Client.TemplateConfig.Wait.Min)
			must.Eq(t, pointer.Of(10*time.Minute), cfg.Client.TemplateConfig.Wait.Max)

			must.NotNil(t, cfg.Client.TemplateConfig.WaitBounds)
			must.Eq(t, pointer.Of(1*time.Second), cfg.Client.TemplateConfig.WaitBounds.Min)
			must.Eq(t, pointer.Of(10*time.Hour), cfg.Client.TemplateConfig.WaitBounds.Max)

			must.NotNil(t, cfg.Client.TemplateConfig.ConsulRetry)
			must.Eq(t, 6, *cfg.Client.TemplateConfig.ConsulRetry.Attempts)
			must.Eq(t, pointer.Of(550*time.Millisecond), cfg.Client.TemplateConfig.ConsulRetry.Backoff)
			must.Eq(t, pointer.Of(10*time.Minute), cfg.Client.TemplateConfig.ConsulRetry.MaxBackoff)

			must.NotNil(t, cfg.Client.TemplateConfig.VaultRetry)
			must.Eq(t, 6, *cfg.Client.TemplateConfig.VaultRetry.Attempts)
			must.Eq(t, pointer.Of(550*time.Millisecond), cfg.Client.TemplateConfig.VaultRetry.Backoff)
			must.Eq(t, pointer.Of(10*time.Minute), cfg.Client.TemplateConfig.VaultRetry.MaxBackoff)

			must.NotNil(t, cfg.Client.TemplateConfig.NomadRetry)
			must.Eq(t, 6, *cfg.Client.TemplateConfig.NomadRetry.Attempts)
			must.Eq(t, pointer.Of(550*time.Millisecond), cfg.Client.TemplateConfig.NomadRetry.Backoff)
			must.Eq(t, pointer.Of(10*time.Minute), cfg.Client.TemplateConfig.NomadRetry.MaxBackoff)
		})
	}
}
