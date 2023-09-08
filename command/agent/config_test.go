// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/nomad/ci"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

var (
	// trueValue/falseValue are used to get a pointer to a boolean
	trueValue  = true
	falseValue = false
)

func TestConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	c0 := &Config{}

	c1 := &Config{
		Telemetry:      &Telemetry{},
		Client:         &ClientConfig{},
		Server:         &ServerConfig{},
		ACL:            &ACLConfig{},
		Audit:          &config.AuditConfig{},
		Ports:          &Ports{},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		Vault:          &config.VaultConfig{},
		Consul:         &config.ConsulConfig{},
		Sentinel:       &config.SentinelConfig{},
		Autopilot:      &config.AutopilotConfig{},
	}

	c2 := &Config{
		Region:                    "global",
		Datacenter:                "dc1",
		NodeName:                  "node1",
		DataDir:                   "/tmp/dir1",
		PluginDir:                 "/tmp/pluginDir1",
		LogLevel:                  "INFO",
		LogJson:                   false,
		EnableDebug:               false,
		LeaveOnInt:                false,
		LeaveOnTerm:               false,
		EnableSyslog:              false,
		SyslogFacility:            "local0.info",
		DisableUpdateCheck:        pointer.Of(false),
		DisableAnonymousSignature: false,
		BindAddr:                  "127.0.0.1",
		Telemetry: &Telemetry{
			StatsiteAddr:                       "127.0.0.1:8125",
			StatsdAddr:                         "127.0.0.1:8125",
			DataDogAddr:                        "127.0.0.1:8125",
			DataDogTags:                        []string{"cat1:tag1", "cat2:tag2"},
			PrometheusMetrics:                  true,
			DisableHostname:                    false,
			CirconusAPIToken:                   "0",
			CirconusAPIApp:                     "nomadic",
			CirconusAPIURL:                     "http://api.circonus.com/v2",
			CirconusSubmissionInterval:         "60s",
			CirconusCheckSubmissionURL:         "https://someplace.com/metrics",
			CirconusCheckID:                    "0",
			CirconusCheckForceMetricActivation: "true",
			CirconusCheckInstanceID:            "node1:nomadic",
			CirconusCheckSearchTag:             "service:nomadic",
			CirconusCheckDisplayName:           "node1:nomadic",
			CirconusCheckTags:                  "cat1:tag1,cat2:tag2",
			CirconusBrokerID:                   "0",
			CirconusBrokerSelectTag:            "dc:dc1",
			PrefixFilter:                       []string{"filter1", "filter2"},
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
		},
		Client: &ClientConfig{
			Enabled:   false,
			StateDir:  "/tmp/state1",
			AllocDir:  "/tmp/alloc1",
			NodeClass: "class1",
			NodePool:  "dev",
			Options: map[string]string{
				"foo": "bar",
			},
			NetworkSpeed:      100,
			CpuCompute:        100,
			MinDynamicPort:    10001,
			MaxDynamicPort:    10002,
			MemoryMB:          100,
			MaxKillTimeout:    "20s",
			ClientMaxPort:     19996,
			DisableRemoteExec: false,
			TemplateConfig: &client.ClientTemplateConfig{
				FunctionDenylist: client.DefaultTemplateFunctionDenylist,
				DisableSandbox:   false,
			},
			Reserved: &Resources{
				CPU:           10,
				MemoryMB:      10,
				DiskMB:        10,
				ReservedPorts: "1,10-30,55",
			},
			NomadServiceDiscovery: pointer.Of(false),
		},
		Server: &ServerConfig{
			Enabled:                false,
			AuthoritativeRegion:    "global",
			BootstrapExpect:        1,
			DataDir:                "/tmp/data1",
			ProtocolVersion:        1,
			RaftProtocol:           1,
			RaftMultiplier:         pointer.Of(5),
			NumSchedulers:          pointer.Of(1),
			NodeGCThreshold:        "1h",
			BatchEvalGCThreshold:   "4h",
			HeartbeatGrace:         30 * time.Second,
			MinHeartbeatTTL:        30 * time.Second,
			MaxHeartbeatsPerSecond: 30.0,
			RedundancyZone:         "foo",
			UpgradeVersion:         "foo",
			EnableEventBroker:      pointer.Of(false),
			EventBufferSize:        pointer.Of(0),
			PlanRejectionTracker: &PlanRejectionTracker{
				Enabled:       pointer.Of(true),
				NodeThreshold: 100,
				NodeWindow:    11 * time.Minute,
			},
		},
		ACL: &ACLConfig{
			Enabled:               true,
			TokenTTL:              60 * time.Second,
			PolicyTTL:             60 * time.Second,
			RoleTTL:               60 * time.Second,
			TokenMinExpirationTTL: 60 * time.Second,
			TokenMaxExpirationTTL: 60 * time.Second,
			ReplicationToken:      "foo",
		},
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses: &Addresses{
			HTTP: "127.0.0.1",
			RPC:  "127.0.0.1",
			Serf: "127.0.0.1",
		},
		AdvertiseAddrs: &AdvertiseAddrs{
			RPC:  "127.0.0.1",
			Serf: "127.0.0.1",
		},
		HTTPAPIResponseHeaders: map[string]string{
			"Access-Control-Allow-Origin": "*",
		},
		Vault: &config.VaultConfig{
			Name:                 "default",
			Token:                "1",
			AllowUnauthenticated: &falseValue,
			TaskTokenTTL:         "1",
			Addr:                 "1",
			TLSCaFile:            "1",
			TLSCaPath:            "1",
			TLSCertFile:          "1",
			TLSKeyFile:           "1",
			TLSSkipVerify:        &falseValue,
			TLSServerName:        "1",
		},
		Vaults: map[string]*config.VaultConfig{
			"default": {
				Name:                 "default",
				Token:                "1",
				AllowUnauthenticated: &falseValue,
				TaskTokenTTL:         "1",
				Addr:                 "1",
				TLSCaFile:            "1",
				TLSCaPath:            "1",
				TLSCertFile:          "1",
				TLSKeyFile:           "1",
				TLSSkipVerify:        &falseValue,
				TLSServerName:        "1",
			},
		},
		Consul: &config.ConsulConfig{
			ServerServiceName:    "1",
			ClientServiceName:    "1",
			AutoAdvertise:        &falseValue,
			Addr:                 "1",
			AllowUnauthenticated: &falseValue,
			Timeout:              1 * time.Second,
			Token:                "1",
			Auth:                 "1",
			EnableSSL:            &falseValue,
			VerifySSL:            &falseValue,
			CAFile:               "1",
			CertFile:             "1",
			KeyFile:              "1",
			ServerAutoJoin:       &falseValue,
			ClientAutoJoin:       &falseValue,
			ChecksUseAdvertise:   &falseValue,
		},
		Consuls: map[string]*config.ConsulConfig{
			"default": {
				ServerServiceName:    "1",
				ClientServiceName:    "1",
				AutoAdvertise:        &falseValue,
				Addr:                 "1",
				AllowUnauthenticated: &falseValue,
				Timeout:              1 * time.Second,
				Token:                "1",
				Auth:                 "1",
				EnableSSL:            &falseValue,
				VerifySSL:            &falseValue,
				CAFile:               "1",
				CertFile:             "1",
				KeyFile:              "1",
				ServerAutoJoin:       &falseValue,
				ClientAutoJoin:       &falseValue,
				ChecksUseAdvertise:   &falseValue,
			},
		},
		Autopilot: &config.AutopilotConfig{
			CleanupDeadServers:      &falseValue,
			ServerStabilizationTime: 1 * time.Second,
			LastContactThreshold:    1 * time.Second,
			MaxTrailingLogs:         1,
			MinQuorum:               1,
			EnableRedundancyZones:   &falseValue,
			DisableUpgradeMigration: &falseValue,
			EnableCustomUpgrades:    &falseValue,
		},
		Plugins: []*config.PluginConfig{
			{
				Name: "docker",
				Args: []string{"foo"},
				Config: map[string]interface{}{
					"bar": 1,
				},
			},
		},
	}

	c3 := &Config{
		Region:                    "global",
		Datacenter:                "dc2",
		NodeName:                  "node2",
		DataDir:                   "/tmp/dir2",
		PluginDir:                 "/tmp/pluginDir2",
		LogLevel:                  "DEBUG",
		LogJson:                   true,
		EnableDebug:               true,
		LeaveOnInt:                true,
		LeaveOnTerm:               true,
		EnableSyslog:              true,
		SyslogFacility:            "local0.debug",
		DisableUpdateCheck:        pointer.Of(true),
		DisableAnonymousSignature: true,
		BindAddr:                  "127.0.0.2",
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
		},
		Telemetry: &Telemetry{
			StatsiteAddr:                       "127.0.0.2:8125",
			StatsdAddr:                         "127.0.0.2:8125",
			DataDogAddr:                        "127.0.0.1:8125",
			DataDogTags:                        []string{"cat1:tag1", "cat2:tag2"},
			PrometheusMetrics:                  true,
			DisableHostname:                    true,
			PublishNodeMetrics:                 true,
			PublishAllocationMetrics:           true,
			CirconusAPIToken:                   "1",
			CirconusAPIApp:                     "nomad",
			CirconusAPIURL:                     "https://api.circonus.com/v2",
			CirconusSubmissionInterval:         "10s",
			CirconusCheckSubmissionURL:         "https://example.com/metrics",
			CirconusCheckID:                    "1",
			CirconusCheckForceMetricActivation: "false",
			CirconusCheckInstanceID:            "node2:nomad",
			CirconusCheckSearchTag:             "service:nomad",
			CirconusCheckDisplayName:           "node2:nomad",
			CirconusCheckTags:                  "cat1:tag1,cat2:tag2",
			CirconusBrokerID:                   "1",
			CirconusBrokerSelectTag:            "dc:dc2",
			PrefixFilter:                       []string{"prefix1", "prefix2"},
			DisableDispatchedJobSummaryMetrics: true,
			DisableRPCRateMetricsLabels:        true,
			FilterDefault:                      pointer.Of(false),
		},
		Client: &ClientConfig{
			Enabled:   true,
			StateDir:  "/tmp/state2",
			AllocDir:  "/tmp/alloc2",
			NodeClass: "class2",
			NodePool:  "dev",
			Servers:   []string{"server2"},
			Meta: map[string]string{
				"baz": "zip",
			},
			Options: map[string]string{
				"foo": "bar",
				"baz": "zip",
			},
			ChrootEnv:         map[string]string{},
			ClientMaxPort:     20000,
			ClientMinPort:     22000,
			NetworkSpeed:      105,
			CpuCompute:        105,
			MinDynamicPort:    10002,
			MaxDynamicPort:    10003,
			MemoryMB:          105,
			MaxKillTimeout:    "50s",
			DisableRemoteExec: false,
			TemplateConfig: &client.ClientTemplateConfig{
				FunctionDenylist: client.DefaultTemplateFunctionDenylist,
				DisableSandbox:   false,
			},
			Reserved: &Resources{
				CPU:           15,
				MemoryMB:      15,
				DiskMB:        15,
				ReservedPorts: "2,10-30,55",
			},
			GCInterval:            6 * time.Second,
			GCParallelDestroys:    6,
			GCDiskUsageThreshold:  71,
			GCInodeUsageThreshold: 86,
			NomadServiceDiscovery: pointer.Of(false),
		},
		Server: &ServerConfig{
			Enabled:                true,
			AuthoritativeRegion:    "global2",
			BootstrapExpect:        2,
			DataDir:                "/tmp/data2",
			ProtocolVersion:        2,
			RaftProtocol:           2,
			RaftMultiplier:         pointer.Of(6),
			NumSchedulers:          pointer.Of(2),
			EnabledSchedulers:      []string{structs.JobTypeBatch},
			NodeGCThreshold:        "12h",
			BatchEvalGCThreshold:   "4h",
			HeartbeatGrace:         2 * time.Minute,
			MinHeartbeatTTL:        2 * time.Minute,
			MaxHeartbeatsPerSecond: 200.0,
			RejoinAfterLeave:       true,
			StartJoin:              []string{"1.1.1.1"},
			RetryJoin:              []string{"1.1.1.1"},
			RetryInterval:          time.Second * 10,
			NonVotingServer:        true,
			RedundancyZone:         "bar",
			UpgradeVersion:         "bar",
			EnableEventBroker:      pointer.Of(true),
			EventBufferSize:        pointer.Of(100),
			PlanRejectionTracker: &PlanRejectionTracker{
				Enabled:       pointer.Of(true),
				NodeThreshold: 100,
				NodeWindow:    11 * time.Minute,
			},
			JobMaxPriority:     pointer.Of(200),
			JobDefaultPriority: pointer.Of(100),
		},
		ACL: &ACLConfig{
			Enabled:               true,
			TokenTTL:              20 * time.Second,
			PolicyTTL:             20 * time.Second,
			RoleTTL:               20 * time.Second,
			TokenMinExpirationTTL: 20 * time.Second,
			TokenMaxExpirationTTL: 20 * time.Second,
			ReplicationToken:      "foobar",
		},
		Ports: &Ports{
			HTTP: 20000,
			RPC:  21000,
			Serf: 22000,
		},
		Addresses: &Addresses{
			HTTP: "127.0.0.2",
			RPC:  "127.0.0.2",
			Serf: "127.0.0.2",
		},
		AdvertiseAddrs: &AdvertiseAddrs{
			RPC:  "127.0.0.2",
			Serf: "127.0.0.2",
		},
		HTTPAPIResponseHeaders: map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		},
		Vault: &config.VaultConfig{
			Name:                 "default",
			Token:                "2",
			AllowUnauthenticated: &trueValue,
			TaskTokenTTL:         "2",
			Addr:                 "2",
			TLSCaFile:            "2",
			TLSCaPath:            "2",
			TLSCertFile:          "2",
			TLSKeyFile:           "2",
			TLSSkipVerify:        &trueValue,
			TLSServerName:        "2",
		},
		Vaults: map[string]*config.VaultConfig{
			"default": {
				Name:                 "default",
				Token:                "2",
				AllowUnauthenticated: &trueValue,
				TaskTokenTTL:         "2",
				Addr:                 "2",
				TLSCaFile:            "2",
				TLSCaPath:            "2",
				TLSCertFile:          "2",
				TLSKeyFile:           "2",
				TLSSkipVerify:        &trueValue,
				TLSServerName:        "2",
			},
		},
		Consul: &config.ConsulConfig{
			ServerServiceName:    "2",
			ClientServiceName:    "2",
			AutoAdvertise:        &trueValue,
			Addr:                 "2",
			AllowUnauthenticated: &trueValue,
			Timeout:              2 * time.Second,
			Token:                "2",
			Auth:                 "2",
			EnableSSL:            &trueValue,
			VerifySSL:            &trueValue,
			CAFile:               "2",
			CertFile:             "2",
			KeyFile:              "2",
			ServerAutoJoin:       &trueValue,
			ClientAutoJoin:       &trueValue,
			ChecksUseAdvertise:   &trueValue,
		},
		Consuls: map[string]*config.ConsulConfig{
			"default": {
				ServerServiceName:    "2",
				ClientServiceName:    "2",
				AutoAdvertise:        &trueValue,
				Addr:                 "2",
				AllowUnauthenticated: &trueValue,
				Timeout:              2 * time.Second,
				Token:                "2",
				Auth:                 "2",
				EnableSSL:            &trueValue,
				VerifySSL:            &trueValue,
				CAFile:               "2",
				CertFile:             "2",
				KeyFile:              "2",
				ServerAutoJoin:       &trueValue,
				ClientAutoJoin:       &trueValue,
				ChecksUseAdvertise:   &trueValue,
			},
		},
		Sentinel: &config.SentinelConfig{
			Imports: []*config.SentinelImport{
				{
					Name: "foo",
					Path: "foo",
					Args: []string{"a", "b", "c"},
				},
			},
		},
		Autopilot: &config.AutopilotConfig{
			CleanupDeadServers:      &trueValue,
			ServerStabilizationTime: 2 * time.Second,
			LastContactThreshold:    2 * time.Second,
			MaxTrailingLogs:         2,
			MinQuorum:               2,
			EnableRedundancyZones:   &trueValue,
			DisableUpgradeMigration: &trueValue,
			EnableCustomUpgrades:    &trueValue,
		},
		Plugins: []*config.PluginConfig{
			{
				Name: "docker",
				Args: []string{"bam"},
				Config: map[string]interface{}{
					"baz": 2,
				},
			},
			{
				Name: "exec",
				Args: []string{"arg"},
				Config: map[string]interface{}{
					"config": true,
				},
			},
		},
	}

	result := c0.Merge(c1)
	result = result.Merge(c2)
	result = result.Merge(c3)
	require.Equal(t, c3, result)
}

func TestConfig_ParseConfigFile(t *testing.T) {
	ci.Parallel(t)

	// Fails if the file doesn't exist
	if _, err := ParseConfigFile("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(fh.Name())

	// Invalid content returns error
	if _, err := fh.WriteString("nope;!!!"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := ParseConfigFile(fh.Name()); err == nil {
		t.Fatalf("expected load error, got nothing")
	}

	// Valid content parses successfully
	if err := fh.Truncate(0); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := fh.Seek(0, 0); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := fh.WriteString(`{"region":"west"}`); err != nil {
		t.Fatalf("err: %s", err)
	}

	config, err := ParseConfigFile(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" {
		t.Fatalf("bad region: %q", config.Region)
	}
}

func TestConfig_LoadConfigDir(t *testing.T) {
	ci.Parallel(t)

	// Fails if the dir doesn't exist.
	if _, err := LoadConfigDir("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	dir := t.TempDir()

	// Returns empty config on empty dir
	config, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config == nil {
		t.Fatalf("should not be nil")
	}

	file1 := filepath.Join(dir, "conf1.hcl")
	err = os.WriteFile(file1, []byte(`{"region":"west"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file2 := filepath.Join(dir, "conf2.hcl")
	err = os.WriteFile(file2, []byte(`{"datacenter":"sfo"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file3 := filepath.Join(dir, "conf3.hcl")
	err = os.WriteFile(file3, []byte(`nope;!!!`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Fails if we have a bad config file
	if _, err := LoadConfigDir(dir); err == nil {
		t.Fatalf("expected load error, got nothing")
	}

	if err := os.Remove(file3); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if configs are valid
	config, err = LoadConfigDir(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" || config.Datacenter != "sfo" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestConfig_LoadConfig(t *testing.T) {
	ci.Parallel(t)

	// Fails if the target doesn't exist
	if _, err := LoadConfig("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh.Name())

	if _, err := fh.WriteString(`{"region":"west"}`); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works on a config file
	config, err := LoadConfig(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" {
		t.Fatalf("bad: %#v", config)
	}

	expectedConfigFiles := []string{fh.Name()}
	if !reflect.DeepEqual(config.Files, expectedConfigFiles) {
		t.Errorf("Loaded configs don't match\nExpected\n%+vGot\n%+v\n",
			expectedConfigFiles, config.Files)
	}

	dir := t.TempDir()

	file1 := filepath.Join(dir, "config1.hcl")
	err = os.WriteFile(file1, []byte(`{"datacenter":"sfo"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works on config dir
	config, err = LoadConfig(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Datacenter != "sfo" {
		t.Fatalf("bad: %#v", config)
	}

	expectedConfigFiles = []string{file1}
	if !reflect.DeepEqual(config.Files, expectedConfigFiles) {
		t.Errorf("Loaded configs don't match\nExpected\n%+vGot\n%+v\n",
			expectedConfigFiles, config.Files)
	}
}

func TestConfig_LoadConfigsFileOrder(t *testing.T) {
	ci.Parallel(t)

	config1, err := LoadConfigDir("test-resources/etcnomad")
	if err != nil {
		t.Fatalf("Failed to load config: %s", err)
	}

	config2, err := LoadConfig("test-resources/myconf")
	if err != nil {
		t.Fatalf("Failed to load config: %s", err)
	}

	expected := []string{
		// filepath.FromSlash changes these to backslash \ on Windows
		filepath.FromSlash("test-resources/etcnomad/common.hcl"),
		filepath.FromSlash("test-resources/etcnomad/server.json"),
		filepath.FromSlash("test-resources/myconf"),
	}

	config := config1.Merge(config2)

	if !reflect.DeepEqual(config.Files, expected) {
		t.Errorf("Loaded configs don't match\nwant: %+v\n got: %+v\n",
			expected, config.Files)
	}
}

func TestConfig_Listener(t *testing.T) {
	ci.Parallel(t)

	config := DefaultConfig()

	// Fails on invalid input
	if ln, err := config.Listener("tcp", "nope", 8080); err == nil {
		ln.Close()
		t.Fatalf("expected addr error")
	}
	if ln, err := config.Listener("nope", "127.0.0.1", 8080); err == nil {
		ln.Close()
		t.Fatalf("expected protocol err")
	}
	if ln, err := config.Listener("tcp", "127.0.0.1", -1); err == nil {
		ln.Close()
		t.Fatalf("expected port error")
	}

	// Works with valid inputs
	ports := ci.PortAllocator.Grab(2)

	ln, err := config.Listener("tcp", "127.0.0.1", ports[0])
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	ln.Close()

	if net := ln.Addr().Network(); net != "tcp" {
		t.Fatalf("expected tcp, got: %q", net)
	}
	want := fmt.Sprintf("127.0.0.1:%d", ports[0])
	if addr := ln.Addr().String(); addr != want {
		t.Fatalf("expected %q, got: %q", want, addr)
	}

	// Falls back to default bind address if non provided
	config.BindAddr = "0.0.0.0"
	ln, err = config.Listener("tcp4", "", ports[1])
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	ln.Close()

	want = fmt.Sprintf("0.0.0.0:%d", ports[1])
	if addr := ln.Addr().String(); addr != want {
		t.Fatalf("expected %q, got: %q", want, addr)
	}
}

func TestConfig_DevModeFlag(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		dev         bool
		connect     bool
		expected    *devModeConfig
		expectedErr string
	}{}
	if runtime.GOOS != "linux" {
		cases = []struct {
			dev         bool
			connect     bool
			expected    *devModeConfig
			expectedErr string
		}{
			{false, false, nil, ""},
			{true, false, &devModeConfig{defaultMode: true, connectMode: false}, ""},
			{true, true, nil, "-dev-connect is only supported on linux"},
			{false, true, nil, "-dev-connect is only supported on linux"},
		}
	}
	if runtime.GOOS == "linux" {
		testutil.RequireRoot(t)
		cases = []struct {
			dev         bool
			connect     bool
			expected    *devModeConfig
			expectedErr string
		}{
			{false, false, nil, ""},
			{true, false, &devModeConfig{defaultMode: true, connectMode: false}, ""},
			{true, true, &devModeConfig{defaultMode: true, connectMode: true}, ""},
			{false, true, &devModeConfig{defaultMode: false, connectMode: true}, ""},
		}
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			mode, err := newDevModeConfig(c.dev, c.connect)
			if err != nil && c.expectedErr == "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), c.expectedErr) {
				t.Fatalf("expected %s; got %v", c.expectedErr, err)
			}
			if mode == nil && c.expected != nil {
				t.Fatalf("expected %+v but got nil", c.expected)
			}
			if mode != nil {
				if c.expected.defaultMode != mode.defaultMode ||
					c.expected.connectMode != mode.connectMode {
					t.Fatalf("expected %+v, got %+v", c.expected, mode)
				}
			}
		})
	}
}

// TestConfig_normalizeAddrs_DevMode asserts that normalizeAddrs allows
// advertising localhost in dev mode.
func TestConfig_normalizeAddrs_DevMode(t *testing.T) {
	ci.Parallel(t)

	// allow to advertise 127.0.0.1 if dev-mode is enabled
	c := &Config{
		BindAddr: "127.0.0.1",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		DevMode:        true,
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unable to normalize addresses: %s", err)
	}

	if c.BindAddr != "127.0.0.1" {
		t.Fatalf("expected BindAddr 127.0.0.1, got %s", c.BindAddr)
	}

	if c.normalizedAddrs.HTTP[0] != "127.0.0.1:4646" {
		t.Fatalf("expected HTTP address 127.0.0.1:4646, got %s", c.normalizedAddrs.HTTP)
	}

	if c.normalizedAddrs.RPC != "127.0.0.1:4647" {
		t.Fatalf("expected RPC address 127.0.0.1:4647, got %s", c.normalizedAddrs.RPC)
	}

	if c.normalizedAddrs.Serf != "127.0.0.1:4648" {
		t.Fatalf("expected Serf address 127.0.0.1:4648, got %s", c.normalizedAddrs.Serf)
	}

	if c.AdvertiseAddrs.HTTP != "127.0.0.1:4646" {
		t.Fatalf("expected HTTP advertise address 127.0.0.1:4646, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != "127.0.0.1:4647" {
		t.Fatalf("expected RPC advertise address 127.0.0.1:4647, got %s", c.AdvertiseAddrs.RPC)
	}

	// Client mode, no Serf address defined
	if c.AdvertiseAddrs.Serf != "" {
		t.Fatalf("expected unset Serf advertise address, got %s", c.AdvertiseAddrs.Serf)
	}
}

// TestConfig_normalizeAddrs_NoAdvertise asserts that normalizeAddrs will
// fail if no valid advertise address available in non-dev mode.
func TestConfig_normalizeAddrs_NoAdvertise(t *testing.T) {
	ci.Parallel(t)

	c := &Config{
		BindAddr: "127.0.0.1",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		DevMode:        false,
	}

	if err := c.normalizeAddrs(); err == nil {
		t.Fatalf("expected an error when no valid advertise address is available")
	}

	if c.AdvertiseAddrs.HTTP == "127.0.0.1:4646" {
		t.Fatalf("expected non-localhost HTTP advertise address, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC == "127.0.0.1:4647" {
		t.Fatalf("expected non-localhost RPC advertise address, got %s", c.AdvertiseAddrs.RPC)
	}

	if c.AdvertiseAddrs.Serf == "127.0.0.1:4648" {
		t.Fatalf("expected non-localhost Serf advertise address, got %s", c.AdvertiseAddrs.Serf)
	}
}

// TestConfig_normalizeAddrs_AdvertiseLocalhost asserts localhost can be
// advertised if it's explicitly set in the config.
func TestConfig_normalizeAddrs_AdvertiseLocalhost(t *testing.T) {
	ci.Parallel(t)

	c := &Config{
		BindAddr: "127.0.0.1",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses: &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{
			HTTP: "127.0.0.1",
			RPC:  "127.0.0.1",
			Serf: "127.0.0.1",
		},
		DevMode: false,
		Server:  &ServerConfig{Enabled: true},
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unexpected error when manually setting bind mode: %v", err)
	}

	if c.AdvertiseAddrs.HTTP != "127.0.0.1:4646" {
		t.Errorf("expected localhost HTTP advertise address, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != "127.0.0.1:4647" {
		t.Errorf("expected localhost RPC advertise address, got %s", c.AdvertiseAddrs.RPC)
	}

	if c.AdvertiseAddrs.Serf != "127.0.0.1:4648" {
		t.Errorf("expected localhost Serf advertise address, got %s", c.AdvertiseAddrs.Serf)
	}
}

// TestConfig_normalizeAddrs_IPv6Loopback asserts that an IPv6 loopback address
// is normalized properly. See #2739
func TestConfig_normalizeAddrs_IPv6Loopback(t *testing.T) {
	ci.Parallel(t)

	c := &Config{
		BindAddr: "::1",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
		},
		Addresses: &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{
			HTTP: "::1",
			RPC:  "::1",
		},
		DevMode: false,
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unexpected error when manually setting bind mode: %v", err)
	}

	if c.Addresses.HTTP != "::1" {
		t.Errorf("expected ::1 HTTP address, got %s", c.Addresses.HTTP)
	}

	if c.Addresses.RPC != "::1" {
		t.Errorf("expected ::1 RPC address, got %s", c.Addresses.RPC)
	}

	if c.AdvertiseAddrs.HTTP != "[::1]:4646" {
		t.Errorf("expected [::1] HTTP advertise address, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != "[::1]:4647" {
		t.Errorf("expected [::1] RPC advertise address, got %s", c.AdvertiseAddrs.RPC)
	}
}

// TestConfig_normalizeAddrs_MultipleInterface asserts that normalizeAddrs will
// handle normalizing multiple interfaces in a single protocol.
func TestConfig_normalizeAddrs_MultipleInterfaces(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                    string
		addressConfig           *Addresses
		expectedNormalizedAddrs *NormalizedAddrs
		expectErr               bool
	}{
		{
			name: "multiple http addresses",
			addressConfig: &Addresses{
				HTTP: "127.0.0.1 127.0.0.2",
			},
			expectedNormalizedAddrs: &NormalizedAddrs{
				HTTP: []string{"127.0.0.1:4646", "127.0.0.2:4646"},
				RPC:  "127.0.0.1:4647",
				Serf: "127.0.0.1:4648",
			},
			expectErr: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{
				BindAddr: "127.0.0.1",
				Ports: &Ports{
					HTTP: 4646,
					RPC:  4647,
					Serf: 4648,
				},
				Addresses: tc.addressConfig,
				AdvertiseAddrs: &AdvertiseAddrs{
					HTTP: "127.0.0.1",
					RPC:  "127.0.0.1",
					Serf: "127.0.0.1",
				},
			}
			err := c.normalizeAddrs()
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedNormalizedAddrs, c.normalizedAddrs)
		})
	}
}

func TestConfig_normalizeAddrs(t *testing.T) {
	ci.Parallel(t)

	c := &Config{
		BindAddr: "169.254.1.5",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses: &Addresses{
			HTTP: "169.254.1.10",
		},
		AdvertiseAddrs: &AdvertiseAddrs{
			RPC: "169.254.1.40",
		},
		Server: &ServerConfig{
			Enabled: true,
		},
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unable to normalize addresses: %s", err)
	}

	if c.BindAddr != "169.254.1.5" {
		t.Fatalf("expected BindAddr 169.254.1.5, got %s", c.BindAddr)
	}

	if c.AdvertiseAddrs.HTTP != "169.254.1.10:4646" {
		t.Fatalf("expected HTTP advertise address 169.254.1.10:4646, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != "169.254.1.40:4647" {
		t.Fatalf("expected RPC advertise address 169.254.1.40:4647, got %s", c.AdvertiseAddrs.RPC)
	}

	if c.AdvertiseAddrs.Serf != "169.254.1.5:4648" {
		t.Fatalf("expected Serf advertise address 169.254.1.5:4648, got %s", c.AdvertiseAddrs.Serf)
	}

	c = &Config{
		BindAddr: "{{ GetPrivateIP }}",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses: &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{
			RPC: "{{ GetPrivateIP }}",
		},
		Server: &ServerConfig{
			Enabled: true,
		},
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unable to normalize addresses: %s", err)
	}

	exp := net.JoinHostPort(c.BindAddr, "4646")
	if c.AdvertiseAddrs.HTTP != exp {
		t.Fatalf("expected HTTP advertise address %s, got %s", exp, c.AdvertiseAddrs.HTTP)
	}

	exp = net.JoinHostPort(c.BindAddr, "4647")
	if c.AdvertiseAddrs.RPC != exp {
		t.Fatalf("expected RPC advertise address %s, got %s", exp, c.AdvertiseAddrs.RPC)
	}

	exp = net.JoinHostPort(c.BindAddr, "4648")
	if c.AdvertiseAddrs.Serf != exp {
		t.Fatalf("expected Serf advertise address %s, got %s", exp, c.AdvertiseAddrs.Serf)
	}

	// allow to advertise 127.0.0.1 in non-dev mode, if explicitly configured to do so
	c = &Config{
		BindAddr: "127.0.0.1",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses: &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{
			HTTP: "127.0.0.1:4646",
			RPC:  "127.0.0.1:4647",
			Serf: "127.0.0.1:4648",
		},
		DevMode: false,
		Server: &ServerConfig{
			Enabled: true,
		},
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unable to normalize addresses: %s", err)
	}

	if c.AdvertiseAddrs.HTTP != "127.0.0.1:4646" {
		t.Fatalf("expected HTTP advertise address 127.0.0.1:4646, got %s", c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != "127.0.0.1:4647" {
		t.Fatalf("expected RPC advertise address 127.0.0.1:4647, got %s", c.AdvertiseAddrs.RPC)
	}

	if c.AdvertiseAddrs.RPC != "127.0.0.1:4647" {
		t.Fatalf("expected RPC advertise address 127.0.0.1:4647, got %s", c.AdvertiseAddrs.RPC)
	}
}

func TestConfig_templateNetworkInterface(t *testing.T) {
	ci.Parallel(t)

	// find the first interface
	ifaces, err := sockaddr.GetAllInterfaces()
	if err != nil {
		t.Fatalf("failed to get interfaces: %v", err)
	}
	iface := ifaces[0]
	testCases := []struct {
		name              string
		clientConfig      *ClientConfig
		expectedInterface string
		expectErr         bool
	}{
		{
			name: "empty string",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: "",
			},
			expectedInterface: "",
			expectErr:         false,
		},
		{
			name: "simple string",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: iface.Name,
			},
			expectedInterface: iface.Name,
			expectErr:         false,
		},
		{
			name: "valid interface",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: `{{ GetAllInterfaces | attr "name" }}`,
			},
			expectedInterface: iface.Name,
			expectErr:         false,
		},
		{
			name: "invalid interface",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: `no such interface`,
			},
			expectedInterface: iface.Name,
			expectErr:         true,
		},
		{
			name: "insignificant whitespace",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: `		{{GetAllInterfaces | attr "name" }}`,
			},
			expectedInterface: iface.Name,
			expectErr:         false,
		},
		{
			name: "empty template return",
			clientConfig: &ClientConfig{
				Enabled:          true,
				NetworkInterface: `{{ printf "" }}`,
			},
			expectedInterface: iface.Name,
			expectErr:         true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{
				BindAddr: "127.0.0.1",
				Ports: &Ports{
					HTTP: 4646,
					RPC:  4647,
					Serf: 4648,
				},
				Addresses: &Addresses{},
				AdvertiseAddrs: &AdvertiseAddrs{
					HTTP: "127.0.0.1:4646",
					RPC:  "127.0.0.1:4647",
					Serf: "127.0.0.1:4648",
				},
				DevMode: false,
				Client:  tc.clientConfig,
			}
			err := c.normalizeAddrs()
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.Client.NetworkInterface, tc.expectedInterface)
		})
	}
}

func TestIsMissingPort(t *testing.T) {
	ci.Parallel(t)

	_, _, err := net.SplitHostPort("localhost")
	if missing := isMissingPort(err); !missing {
		t.Errorf("expected missing port error, but got %v", err)
	}
	_, _, err = net.SplitHostPort("localhost:9000")
	if missing := isMissingPort(err); missing {
		t.Errorf("expected no error, but got %v", err)
	}
}

func TestMergeServerJoin(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	{
		retryJoin := []string{"127.0.0.1", "127.0.0.2"}
		startJoin := []string{"127.0.0.1", "127.0.0.2"}
		retryMaxAttempts := 1
		retryInterval := time.Duration(0)

		a := &ServerJoin{
			RetryJoin:        retryJoin,
			StartJoin:        startJoin,
			RetryMaxAttempts: retryMaxAttempts,
			RetryInterval:    time.Duration(retryInterval),
		}
		b := &ServerJoin{}

		result := a.Merge(b)
		require.Equal(result.RetryJoin, retryJoin)
		require.Equal(result.StartJoin, startJoin)
		require.Equal(result.RetryMaxAttempts, retryMaxAttempts)
		require.Equal(result.RetryInterval, retryInterval)
	}
	{
		retryJoin := []string{"127.0.0.1", "127.0.0.2"}
		startJoin := []string{"127.0.0.1", "127.0.0.2"}
		retryMaxAttempts := 1
		retryInterval := time.Duration(0)

		a := &ServerJoin{}
		b := &ServerJoin{
			RetryJoin:        retryJoin,
			StartJoin:        startJoin,
			RetryMaxAttempts: retryMaxAttempts,
			RetryInterval:    time.Duration(retryInterval),
		}

		result := a.Merge(b)
		require.Equal(result.RetryJoin, retryJoin)
		require.Equal(result.StartJoin, startJoin)
		require.Equal(result.RetryMaxAttempts, retryMaxAttempts)
		require.Equal(result.RetryInterval, retryInterval)
	}
	{
		retryJoin := []string{"127.0.0.1", "127.0.0.2"}
		startJoin := []string{"127.0.0.1", "127.0.0.2"}
		retryMaxAttempts := 1
		retryInterval := time.Duration(0)

		var a *ServerJoin
		b := &ServerJoin{
			RetryJoin:        retryJoin,
			StartJoin:        startJoin,
			RetryMaxAttempts: retryMaxAttempts,
			RetryInterval:    time.Duration(retryInterval),
		}

		result := a.Merge(b)
		require.Equal(result.RetryJoin, retryJoin)
		require.Equal(result.StartJoin, startJoin)
		require.Equal(result.RetryMaxAttempts, retryMaxAttempts)
		require.Equal(result.RetryInterval, retryInterval)
	}
	{
		retryJoin := []string{"127.0.0.1", "127.0.0.2"}
		startJoin := []string{"127.0.0.1", "127.0.0.2"}
		retryMaxAttempts := 1
		retryInterval := time.Duration(0)

		a := &ServerJoin{
			RetryJoin:        retryJoin,
			StartJoin:        startJoin,
			RetryMaxAttempts: retryMaxAttempts,
			RetryInterval:    time.Duration(retryInterval),
		}
		var b *ServerJoin

		result := a.Merge(b)
		require.Equal(result.RetryJoin, retryJoin)
		require.Equal(result.StartJoin, startJoin)
		require.Equal(result.RetryMaxAttempts, retryMaxAttempts)
		require.Equal(result.RetryInterval, retryInterval)
	}
	{
		retryJoin := []string{"127.0.0.1", "127.0.0.2"}
		startJoin := []string{"127.0.0.1", "127.0.0.2"}
		retryMaxAttempts := 1
		retryInterval := time.Duration(0)

		a := &ServerJoin{
			RetryJoin: retryJoin,
			StartJoin: startJoin,
		}
		b := &ServerJoin{
			RetryMaxAttempts: retryMaxAttempts,
			RetryInterval:    time.Duration(retryInterval),
		}

		result := a.Merge(b)
		require.Equal(result.RetryJoin, retryJoin)
		require.Equal(result.StartJoin, startJoin)
		require.Equal(result.RetryMaxAttempts, retryMaxAttempts)
		require.Equal(result.RetryInterval, retryInterval)
	}
}

func TestTelemetry_PrefixFilters(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		in       []string
		expAllow []string
		expBlock []string
		expErr   bool
	}{
		{
			in:       []string{"+foo"},
			expAllow: []string{"foo"},
		},
		{
			in:       []string{"-foo"},
			expBlock: []string{"foo"},
		},
		{
			in:       []string{"+a.b.c", "-x.y.z"},
			expAllow: []string{"a.b.c"},
			expBlock: []string{"x.y.z"},
		},
		{
			in:     []string{"+foo", "bad", "-bar"},
			expErr: true,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("PrefixCase%d", i), func(t *testing.T) {
			require := require.New(t)
			tel := &Telemetry{
				PrefixFilter: c.in,
			}

			allow, block, err := tel.PrefixFilters()
			require.Exactly(c.expAllow, allow)
			require.Exactly(c.expBlock, block)
			require.Equal(c.expErr, err != nil)
		})
	}
}

func TestTelemetry_Parse(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	dir := t.TempDir()

	file1 := filepath.Join(dir, "config1.hcl")
	err := os.WriteFile(file1, []byte(`telemetry{
		prefix_filter = ["+nomad.raft"]
		filter_default = false
		disable_dispatched_job_summary_metrics = true
		disable_rpc_rate_metrics_labels = true
	}`), 0600)
	require.NoError(err)

	// Works on config dir
	config, err := LoadConfig(dir)
	require.NoError(err)

	require.False(*config.Telemetry.FilterDefault)
	require.Exactly([]string{"+nomad.raft"}, config.Telemetry.PrefixFilter)
	require.True(config.Telemetry.DisableDispatchedJobSummaryMetrics)
	require.True(config.Telemetry.DisableRPCRateMetricsLabels)
}

func TestEventBroker_Parse(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	{
		a := &ServerConfig{
			EnableEventBroker: pointer.Of(false),
			EventBufferSize:   pointer.Of(0),
		}
		b := DefaultConfig().Server
		b.EnableEventBroker = nil
		b.EventBufferSize = nil

		result := a.Merge(b)
		require.Equal(false, *result.EnableEventBroker)
		require.Equal(0, *result.EventBufferSize)
	}

	{
		a := &ServerConfig{
			EnableEventBroker: pointer.Of(true),
			EventBufferSize:   pointer.Of(5000),
		}
		b := DefaultConfig().Server
		b.EnableEventBroker = nil
		b.EventBufferSize = nil

		result := a.Merge(b)
		require.Equal(true, *result.EnableEventBroker)
		require.Equal(5000, *result.EventBufferSize)
	}

	{
		a := &ServerConfig{
			EnableEventBroker: pointer.Of(false),
			EventBufferSize:   pointer.Of(0),
		}
		b := DefaultConfig().Server
		b.EnableEventBroker = pointer.Of(true)
		b.EventBufferSize = pointer.Of(20000)

		result := a.Merge(b)
		require.Equal(true, *result.EnableEventBroker)
		require.Equal(20000, *result.EventBufferSize)
	}
}

func TestConfig_LoadConsulTemplateConfig(t *testing.T) {
	ci.Parallel(t)

	defaultConfig := DefaultConfig()
	// Test that loading without template config didn't create load errors
	agentConfig, err := LoadConfig("test-resources/minimal_client.hcl")
	require.NoError(t, err)

	// Test loading with this config didn't create load errors
	agentConfig, err = LoadConfig("test-resources/client_with_template.hcl")
	require.NoError(t, err)

	agentConfig = defaultConfig.Merge(agentConfig)

	clientAgent := Agent{config: agentConfig}
	clientConfig, err := clientAgent.clientConfig()
	require.NoError(t, err)

	templateConfig := clientConfig.TemplateConfig

	// Make sure all fields to test are set
	require.NotNil(t, templateConfig.BlockQueryWaitTime)
	require.NotNil(t, templateConfig.MaxStale)
	require.NotNil(t, templateConfig.Wait)
	require.NotNil(t, templateConfig.WaitBounds)
	require.NotNil(t, templateConfig.ConsulRetry)
	require.NotNil(t, templateConfig.VaultRetry)
	require.NotNil(t, templateConfig.NomadRetry)

	// Direct properties
	require.Equal(t, 300*time.Second, *templateConfig.MaxStale)
	require.Equal(t, 90*time.Second, *templateConfig.BlockQueryWaitTime)
	// Wait
	require.Equal(t, 2*time.Second, *templateConfig.Wait.Min)
	require.Equal(t, 60*time.Second, *templateConfig.Wait.Max)
	// WaitBounds
	require.Equal(t, 2*time.Second, *templateConfig.WaitBounds.Min)
	require.Equal(t, 60*time.Second, *templateConfig.WaitBounds.Max)
	// Consul Retry
	require.NotNil(t, templateConfig.ConsulRetry)
	require.Equal(t, 5, *templateConfig.ConsulRetry.Attempts)
	require.Equal(t, 5*time.Second, *templateConfig.ConsulRetry.Backoff)
	require.Equal(t, 10*time.Second, *templateConfig.ConsulRetry.MaxBackoff)
	// Vault Retry
	require.NotNil(t, templateConfig.VaultRetry)
	require.Equal(t, 10, *templateConfig.VaultRetry.Attempts)
	require.Equal(t, 15*time.Second, *templateConfig.VaultRetry.Backoff)
	require.Equal(t, 20*time.Second, *templateConfig.VaultRetry.MaxBackoff)
	// Nomad Retry
	require.NotNil(t, templateConfig.NomadRetry)
	require.Equal(t, 15, *templateConfig.NomadRetry.Attempts)
	require.Equal(t, 20*time.Second, *templateConfig.NomadRetry.Backoff)
	require.Equal(t, 25*time.Second, *templateConfig.NomadRetry.MaxBackoff)
}

func TestConfig_LoadConsulTemplate_FunctionDenylist(t *testing.T) {
	cases := []struct {
		File     string
		Expected *client.ClientTemplateConfig
	}{
		{
			"test-resources/minimal_client.hcl",
			nil,
		},
		{
			"test-resources/client_with_basic_template.json",
			&client.ClientTemplateConfig{
				DisableSandbox:   true,
				FunctionDenylist: []string{},
			},
		},
		{
			"test-resources/client_with_basic_template.hcl",
			&client.ClientTemplateConfig{
				DisableSandbox:   true,
				FunctionDenylist: []string{},
			},
		},
		{
			"test-resources/client_with_function_denylist.hcl",
			&client.ClientTemplateConfig{
				DisableSandbox:   false,
				FunctionDenylist: []string{"foo"},
			},
		},
		{
			"test-resources/client_with_function_denylist_empty.hcl",
			&client.ClientTemplateConfig{
				DisableSandbox:   false,
				FunctionDenylist: []string{},
			},
		},
		{
			"test-resources/client_with_function_denylist_empty_string.hcl",
			&client.ClientTemplateConfig{
				DisableSandbox:   true,
				FunctionDenylist: []string{""},
			},
		},
		{
			"test-resources/client_with_function_denylist_empty_string.json",
			&client.ClientTemplateConfig{
				DisableSandbox:   true,
				FunctionDenylist: []string{""},
			},
		},
		{
			"test-resources/client_with_function_denylist_nil.hcl",
			&client.ClientTemplateConfig{
				DisableSandbox: true,
			},
		},
		{
			"test-resources/client_with_empty_template.hcl",
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			agentConfig, err := LoadConfig(tc.File)

			require.NoError(t, err)

			templateConfig := agentConfig.Client.TemplateConfig
			require.Equal(t, tc.Expected, templateConfig)
		})
	}
}

func TestParseMultipleIPTemplates(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		tmpl        string
		expectedOut []string
		expectErr   bool
	}{
		{
			name:        "deduplicates same ip and preserves order",
			tmpl:        "127.0.0.1 10.0.0.1 127.0.0.1",
			expectedOut: []string{"127.0.0.1", "10.0.0.1"},
			expectErr:   false,
		},
		{
			name:        "includes sockaddr expression",
			tmpl:        "10.0.0.1 {{ GetAllInterfaces | include \"flags\" \"loopback\" | limit 1 | attr \"address\" }} 10.0.0.2",
			expectedOut: []string{"10.0.0.1", "127.0.0.1", "10.0.0.2"},
			expectErr:   false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := parseMultipleIPTemplate(tc.tmpl)
			require.NoError(t, err)
			require.Equal(t, tc.expectedOut, out)
		})
	}
}
