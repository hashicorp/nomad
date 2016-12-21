package agent

import (
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
	c1 := &Config{
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
			Enabled:         false,
			BootstrapExpect: 1,
			DataDir:         "/tmp/data1",
			ProtocolVersion: 1,
			NumSchedulers:   1,
			NodeGCThreshold: "1h",
			HeartbeatGrace:  "30s",
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
			ServerServiceName: "1",
			ClientServiceName: "1",
			AutoAdvertise:     false,
			Addr:              "1",
			Timeout:           1 * time.Second,
			Token:             "1",
			Auth:              "1",
			EnableSSL:         false,
			VerifySSL:         false,
			CAFile:            "1",
			CertFile:          "1",
			KeyFile:           "1",
			ServerAutoJoin:    false,
			ClientAutoJoin:    false,
		},
	}

	c2 := &Config{
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
			MaxKillTimeout: "50s",
			Reserved: &Resources{
				CPU:                 15,
				MemoryMB:            15,
				DiskMB:              15,
				IOPS:                15,
				ReservedPorts:       "2,10-30,55",
				ParsedReservedPorts: []int{1, 2, 3},
			},
		},
		Server: &ServerConfig{
			Enabled:           true,
			BootstrapExpect:   2,
			DataDir:           "/tmp/data2",
			ProtocolVersion:   2,
			NumSchedulers:     2,
			EnabledSchedulers: []string{structs.JobTypeBatch},
			NodeGCThreshold:   "12h",
			HeartbeatGrace:    "2m",
			RejoinAfterLeave:  true,
			StartJoin:         []string{"1.1.1.1"},
			RetryJoin:         []string{"1.1.1.1"},
			RetryInterval:     "10s",
			retryInterval:     time.Second * 10,
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
			ServerServiceName: "2",
			ClientServiceName: "2",
			AutoAdvertise:     true,
			Addr:              "2",
			Timeout:           2 * time.Second,
			Token:             "2",
			Auth:              "2",
			EnableSSL:         true,
			VerifySSL:         true,
			CAFile:            "2",
			CertFile:          "2",
			KeyFile:           "2",
			ServerAutoJoin:    true,
			ClientAutoJoin:    true,
		},
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, c2) {
		t.Fatalf("bad:\n%#v\n%#v", result, c2)
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
