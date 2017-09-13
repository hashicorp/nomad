package agent

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

var (
	// trueValue/falseValue are used to get a pointer to a boolean
	trueValue  = true
	falseValue = false
)

func TestConfig_Merge(t *testing.T) {
	c0 := &Config{}

	c1 := &Config{
		Telemetry:      &Telemetry{},
		Client:         &ClientConfig{},
		Server:         &ServerConfig{},
		ACL:            &ACLConfig{},
		Ports:          &Ports{},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		Atlas:          &AtlasConfig{},
		Vault:          &config.VaultConfig{},
		Consul:         &config.ConsulConfig{},
	}

	c2 := &Config{
		Region:                    "global",
		Datacenter:                "dc1",
		NodeName:                  "node1",
		DataDir:                   "/tmp/dir1",
		LogLevel:                  "INFO",
		EnableDebug:               false,
		LeaveOnInt:                false,
		LeaveOnTerm:               false,
		EnableSyslog:              false,
		SyslogFacility:            "local0.info",
		DisableUpdateCheck:        false,
		DisableAnonymousSignature: false,
		BindAddr:                  "127.0.0.1",
		Telemetry: &Telemetry{
			StatsiteAddr:                       "127.0.0.1:8125",
			StatsdAddr:                         "127.0.0.1:8125",
			DataDogAddr:                        "127.0.0.1:8125",
			PrometheusMetrics:                  true,
			DisableHostname:                    false,
			DisableTaggedMetrics:               true,
			BackwardsCompatibleMetrics:         true,
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
		},
		Client: &ClientConfig{
			Enabled:   false,
			StateDir:  "/tmp/state1",
			AllocDir:  "/tmp/alloc1",
			NodeClass: "class1",
			Options: map[string]string{
				"foo": "bar",
			},
			NetworkSpeed:   100,
			CpuCompute:     100,
			MaxKillTimeout: "20s",
			ClientMaxPort:  19996,
			Reserved: &Resources{
				CPU:                 10,
				MemoryMB:            10,
				DiskMB:              10,
				IOPS:                10,
				ReservedPorts:       "1,10-30,55",
				ParsedReservedPorts: []int{1, 2, 4},
			},
		},
		Server: &ServerConfig{
			Enabled:                false,
			AuthoritativeRegion:    "global",
			BootstrapExpect:        1,
			DataDir:                "/tmp/data1",
			ProtocolVersion:        1,
			NumSchedulers:          1,
			NodeGCThreshold:        "1h",
			HeartbeatGrace:         30 * time.Second,
			MinHeartbeatTTL:        30 * time.Second,
			MaxHeartbeatsPerSecond: 30.0,
		},
		ACL: &ACLConfig{
			Enabled:          true,
			TokenTTL:         60 * time.Second,
			PolicyTTL:        60 * time.Second,
			ReplicationToken: "foo",
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
		Atlas: &AtlasConfig{
			Infrastructure: "hashicorp/test1",
			Token:          "abc",
			Join:           false,
			Endpoint:       "foo",
		},
		HTTPAPIResponseHeaders: map[string]string{
			"Access-Control-Allow-Origin": "*",
		},
		Vault: &config.VaultConfig{
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
		Consul: &config.ConsulConfig{
			ServerServiceName:  "1",
			ClientServiceName:  "1",
			AutoAdvertise:      &falseValue,
			Addr:               "1",
			Timeout:            1 * time.Second,
			Token:              "1",
			Auth:               "1",
			EnableSSL:          &falseValue,
			VerifySSL:          &falseValue,
			CAFile:             "1",
			CertFile:           "1",
			KeyFile:            "1",
			ServerAutoJoin:     &falseValue,
			ClientAutoJoin:     &falseValue,
			ChecksUseAdvertise: &falseValue,
		},
	}

	c3 := &Config{
		Region:                    "region2",
		Datacenter:                "dc2",
		NodeName:                  "node2",
		DataDir:                   "/tmp/dir2",
		LogLevel:                  "DEBUG",
		EnableDebug:               true,
		LeaveOnInt:                true,
		LeaveOnTerm:               true,
		EnableSyslog:              true,
		SyslogFacility:            "local0.debug",
		DisableUpdateCheck:        true,
		DisableAnonymousSignature: true,
		BindAddr:                  "127.0.0.2",
		Telemetry: &Telemetry{
			StatsiteAddr:                       "127.0.0.2:8125",
			StatsdAddr:                         "127.0.0.2:8125",
			DataDogAddr:                        "127.0.0.1:8125",
			PrometheusMetrics:                  true,
			DisableHostname:                    true,
			PublishNodeMetrics:                 true,
			PublishAllocationMetrics:           true,
			DisableTaggedMetrics:               true,
			BackwardsCompatibleMetrics:         true,
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
		},
		Client: &ClientConfig{
			Enabled:   true,
			StateDir:  "/tmp/state2",
			AllocDir:  "/tmp/alloc2",
			NodeClass: "class2",
			Servers:   []string{"server2"},
			Meta: map[string]string{
				"baz": "zip",
			},
			Options: map[string]string{
				"foo": "bar",
				"baz": "zip",
			},
			ChrootEnv:      map[string]string{},
			ClientMaxPort:  20000,
			ClientMinPort:  22000,
			NetworkSpeed:   105,
			CpuCompute:     105,
			MaxKillTimeout: "50s",
			Reserved: &Resources{
				CPU:                 15,
				MemoryMB:            15,
				DiskMB:              15,
				IOPS:                15,
				ReservedPorts:       "2,10-30,55",
				ParsedReservedPorts: []int{1, 2, 3},
			},
			GCInterval:            6 * time.Second,
			GCParallelDestroys:    6,
			GCDiskUsageThreshold:  71,
			GCInodeUsageThreshold: 86,
		},
		Server: &ServerConfig{
			Enabled:                true,
			AuthoritativeRegion:    "global2",
			BootstrapExpect:        2,
			DataDir:                "/tmp/data2",
			ProtocolVersion:        2,
			NumSchedulers:          2,
			EnabledSchedulers:      []string{structs.JobTypeBatch},
			NodeGCThreshold:        "12h",
			HeartbeatGrace:         2 * time.Minute,
			MinHeartbeatTTL:        2 * time.Minute,
			MaxHeartbeatsPerSecond: 200.0,
			RejoinAfterLeave:       true,
			StartJoin:              []string{"1.1.1.1"},
			RetryJoin:              []string{"1.1.1.1"},
			RetryInterval:          "10s",
			retryInterval:          time.Second * 10,
		},
		ACL: &ACLConfig{
			Enabled:          true,
			TokenTTL:         20 * time.Second,
			PolicyTTL:        20 * time.Second,
			ReplicationToken: "foobar",
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
		Atlas: &AtlasConfig{
			Infrastructure: "hashicorp/test2",
			Token:          "xyz",
			Join:           true,
			Endpoint:       "bar",
		},
		HTTPAPIResponseHeaders: map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		},
		Vault: &config.VaultConfig{
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
		Consul: &config.ConsulConfig{
			ServerServiceName:  "2",
			ClientServiceName:  "2",
			AutoAdvertise:      &trueValue,
			Addr:               "2",
			Timeout:            2 * time.Second,
			Token:              "2",
			Auth:               "2",
			EnableSSL:          &trueValue,
			VerifySSL:          &trueValue,
			CAFile:             "2",
			CertFile:           "2",
			KeyFile:            "2",
			ServerAutoJoin:     &trueValue,
			ClientAutoJoin:     &trueValue,
			ChecksUseAdvertise: &trueValue,
		},
	}

	result := c0.Merge(c1)
	result = result.Merge(c2)
	result = result.Merge(c3)
	if !reflect.DeepEqual(result, c3) {
		t.Fatalf("bad:\n%#v\n%#v", result, c3)
	}
}

func TestConfig_ParseConfigFile(t *testing.T) {
	// Fails if the file doesn't exist
	if _, err := ParseConfigFile("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := ioutil.TempFile("", "nomad")
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
	// Fails if the dir doesn't exist.
	if _, err := LoadConfigDir("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)

	// Returns empty config on empty dir
	config, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config == nil {
		t.Fatalf("should not be nil")
	}

	file1 := filepath.Join(dir, "conf1.hcl")
	err = ioutil.WriteFile(file1, []byte(`{"region":"west"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file2 := filepath.Join(dir, "conf2.hcl")
	err = ioutil.WriteFile(file2, []byte(`{"datacenter":"sfo"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file3 := filepath.Join(dir, "conf3.hcl")
	err = ioutil.WriteFile(file3, []byte(`nope;!!!`), 0600)
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
	// Fails if the target doesn't exist
	if _, err := LoadConfig("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := ioutil.TempFile("", "nomad")
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

	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "config1.hcl")
	err = ioutil.WriteFile(file1, []byte(`{"datacenter":"sfo"}`), 0600)
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
	ln, err := config.Listener("tcp", "127.0.0.1", 24000)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	ln.Close()

	if net := ln.Addr().Network(); net != "tcp" {
		t.Fatalf("expected tcp, got: %q", net)
	}
	if addr := ln.Addr().String(); addr != "127.0.0.1:24000" {
		t.Fatalf("expected 127.0.0.1:4646, got: %q", addr)
	}

	// Falls back to default bind address if non provided
	config.BindAddr = "0.0.0.0"
	ln, err = config.Listener("tcp4", "", 24000)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	ln.Close()

	if addr := ln.Addr().String(); addr != "0.0.0.0:24000" {
		t.Fatalf("expected 0.0.0.0:24000, got: %q", addr)
	}
}

// TestConfig_normalizeAddrs_DevMode asserts that normalizeAddrs allows
// advertising localhost in dev mode.
func TestConfig_normalizeAddrs_DevMode(t *testing.T) {
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

	if c.normalizedAddrs.HTTP != "127.0.0.1:4646" {
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

func TestConfig_normalizeAddrs(t *testing.T) {
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
			RPC: "{{ GetPrivateIP }}:8888",
		},
		Server: &ServerConfig{
			Enabled: true,
		},
	}

	if err := c.normalizeAddrs(); err != nil {
		t.Fatalf("unable to normalize addresses: %s", err)
	}

	if c.AdvertiseAddrs.HTTP != fmt.Sprintf("%s:4646", c.BindAddr) {
		t.Fatalf("expected HTTP advertise address %s:4646, got %s", c.BindAddr, c.AdvertiseAddrs.HTTP)
	}

	if c.AdvertiseAddrs.RPC != fmt.Sprintf("%s:8888", c.BindAddr) {
		t.Fatalf("expected RPC advertise address %s:8888, got %s", c.BindAddr, c.AdvertiseAddrs.RPC)
	}

	if c.AdvertiseAddrs.Serf != fmt.Sprintf("%s:4648", c.BindAddr) {
		t.Fatalf("expected Serf advertise address %s:4648, got %s", c.BindAddr, c.AdvertiseAddrs.Serf)
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

func TestResources_ParseReserved(t *testing.T) {
	cases := []struct {
		Input  string
		Parsed []int
		Err    bool
	}{
		{
			"1,2,3",
			[]int{1, 2, 3},
			false,
		},
		{
			"3,1,2,1,2,3,1-3",
			[]int{1, 2, 3},
			false,
		},
		{
			"3-1",
			nil,
			true,
		},
		{
			"1-3,2-4",
			[]int{1, 2, 3, 4},
			false,
		},
		{
			"1-3,4,5-5,6,7,8-10",
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			false,
		},
	}

	for i, tc := range cases {
		r := &Resources{ReservedPorts: tc.Input}
		err := r.ParseReserved()
		if (err != nil) != tc.Err {
			t.Fatalf("test case %d: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(r.ParsedReservedPorts, tc.Parsed) {
			t.Fatalf("test case %d: \n\n%#v\n\n%#v", i, r.ParsedReservedPorts, tc.Parsed)
		}

	}
}

func TestIsMissingPort(t *testing.T) {
	_, _, err := net.SplitHostPort("localhost")
	if missing := isMissingPort(err); !missing {
		t.Errorf("expected missing port error, but got %v", err)
	}
	_, _, err = net.SplitHostPort("localhost:9000")
	if missing := isMissingPort(err); missing {
		t.Errorf("expected no error, but got %v", err)
	}
}
