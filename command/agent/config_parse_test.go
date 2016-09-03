package agent

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs/config"
)

func TestConfig_Parse(t *testing.T) {
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
				},
				Server: &ServerConfig{
					Enabled:           true,
					BootstrapExpect:   5,
					DataDir:           "/tmp/data",
					ProtocolVersion:   3,
					NumSchedulers:     2,
					EnabledSchedulers: []string{"test"},
					NodeGCThreshold:   "12h",
					HeartbeatGrace:    "30s",
					RetryJoin:         []string{"1.1.1.1", "2.2.2.2"},
					StartJoin:         []string{"1.1.1.1", "2.2.2.2"},
					RetryInterval:     "15s",
					RejoinAfterLeave:  true,
					RetryMaxAttempts:  3,
				},
				Telemetry: &Telemetry{
					StatsiteAddr:             "127.0.0.1:1234",
					StatsdAddr:               "127.0.0.1:2345",
					DisableHostname:          true,
					CollectionInterval:       "3s",
					collectionInterval:       3 * time.Second,
					PublishAllocationMetrics: true,
					PublishNodeMetrics:       true,
				},
				LeaveOnInt:                true,
				LeaveOnTerm:               true,
				EnableSyslog:              true,
				SyslogFacility:            "LOCAL1",
				DisableUpdateCheck:        true,
				DisableAnonymousSignature: true,
				Atlas: &AtlasConfig{
					Infrastructure: "armon/test",
					Token:          "abcd",
					Join:           true,
					Endpoint:       "127.0.0.1:1234",
				},
				Consul: &config.ConsulConfig{
					ServerServiceName: "nomad",
					ClientServiceName: "nomad-client",
					Addr:              "127.0.0.1:9500",
					Token:             "token1",
					Auth:              "username:pass",
					EnableSSL:         true,
					VerifySSL:         false,
					CAFile:            "/path/to/ca/file",
					CertFile:          "/path/to/cert/file",
					KeyFile:           "/path/to/key/file",
					ServerAutoJoin:    false,
					ClientAutoJoin:    false,
					AutoAdvertise:     false,
				},
				Vault: &config.VaultConfig{
					Addr:                 "127.0.0.1:9500",
					AllowUnauthenticated: true,
					Enabled:              false,
					TLSCaFile:            "/path/to/ca/file",
					TLSCaPath:            "/path/to/ca",
					TLSCertFile:          "/path/to/cert/file",
					TLSKeyFile:           "/path/to/key/file",
					TLSServerName:        "foobar",
					TLSSkipVerify:        true,
					TaskTokenTTL:         "1s",
					Token:                "12345",
				},
				HTTPAPIResponseHeaders: map[string]string{
					"Access-Control-Allow-Origin": "*",
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Logf("Testing parse: %s", tc.File)

		path, err := filepath.Abs(filepath.Join("./config-test-fixtures", tc.File))
		if err != nil {
			t.Fatalf("file: %s\n\n%s", tc.File, err)
			continue
		}

		actual, err := ParseConfigFile(path)
		if (err != nil) != tc.Err {
			t.Fatalf("file: %s\n\n%s", tc.File, err)
			continue
		}

		if !reflect.DeepEqual(actual, tc.Result) {
			t.Fatalf("file: %s\n\n%#v\n\n%#v", tc.File, actual, tc.Result)
		}
	}
}
