// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/version"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &Command{}
}

func TestCommand_Args(t *testing.T) {
	ci.Parallel(t)
	tmpDir := t.TempDir()

	type tcase struct {
		args   []string
		errOut string
	}
	tcases := []tcase{
		{
			[]string{},
			"Must specify either server, client or dev mode for the agent.",
		},
		{
			[]string{"-client", "-data-dir=" + tmpDir, "-bootstrap-expect=1"},
			"Bootstrap requires server mode to be enabled",
		},
		{
			[]string{"-data-dir=" + tmpDir, "-server", "-bootstrap-expect=1"},
			"WARNING: Bootstrap mode enabled!",
		},
		{
			[]string{"-data-dir=" + tmpDir, "-server", "-bootstrap-expect=2"},
			"Number of bootstrap servers should ideally be set to an odd number",
		},
		{
			[]string{"-server"},
			"Must specify \"data_dir\" config option or \"data-dir\" CLI flag",
		},
		{
			[]string{"-client", "-alloc-dir="},
			"Must specify the state, alloc-dir, alloc-mounts-dir and plugin-dir if data-dir is omitted.",
		},
		{
			[]string{"-client", "-data-dir=" + tmpDir, "-meta=invalid..key=inaccessible-value"},
			"Invalid Client.Meta key: invalid..key",
		},
		{
			[]string{"-client", "-data-dir=" + tmpDir, "-meta=.invalid=inaccessible-value"},
			"Invalid Client.Meta key: .invalid",
		},
		{
			[]string{"-client", "-data-dir=" + tmpDir, "-meta=invalid.=inaccessible-value"},
			"Invalid Client.Meta key: invalid.",
		},
		{
			[]string{"-client", "-node-pool=not@valid"},
			"Invalid node pool",
		},
	}
	for _, tc := range tcases {
		// Make a new command. We preemptively close the shutdownCh
		// so that the command exits immediately instead of blocking.
		ui := cli.NewMockUi()
		shutdownCh := make(chan struct{})
		close(shutdownCh)
		cmd := &Command{
			Version:    version.GetVersion(),
			Ui:         ui,
			ShutdownCh: shutdownCh,
		}

		// To prevent test failures on hosts whose hostname resolves to
		// a loopback address, we must append a bind address
		tc.args = append(tc.args, "-bind=169.254.0.1")
		if code := cmd.Run(tc.args); code != 1 {
			t.Fatalf("args: %v\nexit: %d\n", tc.args, code)
		}

		if expect := tc.errOut; expect != "" {
			out := ui.ErrorWriter.String()
			if !strings.Contains(out, expect) {
				t.Fatalf("expect to find %q\n\n%s", expect, out)
			}
		}
	}
}

func TestCommand_MetaConfigValidation(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	tcases := []string{
		"foo..invalid",
		".invalid",
		"invalid.",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err := os.WriteFile(configFile, []byte(`client{
			enabled = true
			meta = {
				"valid" = "yes"
				"`+tc+`" = "kaboom!"
				"nested.var" = "is nested"
				"deeply.nested.var" = "is deeply nested"
			}
    	}`), 0600)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Make a new command. We preemptively close the shutdownCh
		// so that the command exits immediately instead of blocking.
		ui := cli.NewMockUi()
		shutdownCh := make(chan struct{})
		close(shutdownCh)
		cmd := &Command{
			Version:    version.GetVersion(),
			Ui:         ui,
			ShutdownCh: shutdownCh,
		}

		// To prevent test failures on hosts whose hostname resolves to
		// a loopback address, we must append a bind address
		args := []string{"-client", "-data-dir=" + tmpDir, "-config=" + configFile, "-bind=169.254.0.1"}
		if code := cmd.Run(args); code != 1 {
			t.Fatalf("args: %v\nexit: %d\n", args, code)
		}

		expect := "Invalid Client.Meta key: " + tc
		out := ui.ErrorWriter.String()
		if !strings.Contains(out, expect) {
			t.Fatalf("expect to find %q\n\n%s", expect, out)
		}
	}
}

func TestCommand_InvalidCharInDatacenter(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	tcases := []string{
		"char-\\000-in-the-middle",
		"ends-with-\\000",
		"\\000-at-the-beginning",
		"char-*-in-the-middle",
		"ends-with-*",
		"*-at-the-beginning",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err := os.WriteFile(configFile, []byte(`
        datacenter = "`+tc+`"
        client{
			enabled = true
    	}`), 0600)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Make a new command. We preemptively close the shutdownCh
		// so that the command exits immediately instead of blocking.
		ui := cli.NewMockUi()
		shutdownCh := make(chan struct{})
		close(shutdownCh)
		cmd := &Command{
			Version:    version.GetVersion(),
			Ui:         ui,
			ShutdownCh: shutdownCh,
		}

		// To prevent test failures on hosts whose hostname resolves to
		// a loopback address, we must append a bind address
		args := []string{"-client", "-data-dir=" + tmpDir, "-config=" + configFile, "-bind=169.254.0.1"}
		if code := cmd.Run(args); code != 1 {
			t.Fatalf("args: %v\nexit: %d\n", args, code)
		}

		out := ui.ErrorWriter.String()
		exp := "Datacenter contains invalid characters (null or '*')"
		if !strings.Contains(out, exp) {
			t.Fatalf("expect to find %q\n\n%s", exp, out)
		}
	}
}

func TestCommand_NullCharInRegion(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	tcases := []string{
		"char-\\000-in-the-middle",
		"ends-with-\\000",
		"\\000-at-the-beginning",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err := os.WriteFile(configFile, []byte(`
        region = "`+tc+`"
        client{
			enabled = true
    	}`), 0600)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		// Make a new command. We preemptively close the shutdownCh
		// so that the command exits immediately instead of blocking.
		ui := cli.NewMockUi()
		shutdownCh := make(chan struct{})
		close(shutdownCh)
		cmd := &Command{
			Version:    version.GetVersion(),
			Ui:         ui,
			ShutdownCh: shutdownCh,
		}

		// To prevent test failures on hosts whose hostname resolves to
		// a loopback address, we must append a bind address
		args := []string{"-client", "-data-dir=" + tmpDir, "-config=" + configFile, "-bind=169.254.0.1"}
		if code := cmd.Run(args); code != 1 {
			t.Fatalf("args: %v\nexit: %d\n", args, code)
		}

		out := ui.ErrorWriter.String()
		exp := "Region contains invalid characters"
		if !strings.Contains(out, exp) {
			t.Fatalf("expect to find %q\n\n%s", exp, out)
		}
	}
}

// TestIsValidConfig asserts that invalid configurations return false.
func TestIsValidConfig(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name string
		conf Config // merged into DefaultConfig()

		// err should appear in error output; success expected if err
		// is empty
		err string
	}{
		{
			name: "Default",
			conf: Config{
				DataDir: "/tmp",
				Client:  &ClientConfig{Enabled: true},
			},
		},
		{
			name: "NoMode",
			conf: Config{
				Client: &ClientConfig{Enabled: false},
				Server: &ServerConfig{Enabled: false},
			},
			err: "Must specify either",
		},
		{
			name: "InvalidRegion",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
				},
				Region: "Hello\000World",
			},
			err: "Region contains",
		},
		{
			name: "InvalidDatacenter",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
				},
				Datacenter: "Hello\000World",
			},
			err: "Datacenter contains",
		},
		{
			name: "RelativeDir",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
				},
				DataDir: "foo/bar",
			},
			err: "must be given as an absolute",
		},
		{
			name: "InvalidNodePoolChar",
			conf: Config{
				Client: &ClientConfig{
					Enabled:  true,
					NodePool: "not@valid",
				},
			},
			err: "Invalid node pool",
		},
		{
			name: "InvalidNodePoolName",
			conf: Config{
				Client: &ClientConfig{
					Enabled:  true,
					NodePool: structs.NodePoolAll,
				},
			},
			err: "not allowed",
		},
		{
			name: "NegativeMinDynamicPort",
			conf: Config{
				Client: &ClientConfig{
					Enabled:        true,
					MinDynamicPort: -1,
				},
			},
			err: "min_dynamic_port",
		},
		{
			name: "NegativeMaxDynamicPort",
			conf: Config{
				Client: &ClientConfig{
					Enabled:        true,
					MaxDynamicPort: -1,
				},
			},
			err: "max_dynamic_port",
		},
		{
			name: "BigMinDynamicPort",
			conf: Config{
				Client: &ClientConfig{
					Enabled:        true,
					MinDynamicPort: math.MaxInt32,
				},
			},
			err: "min_dynamic_port",
		},
		{
			name: "BigMaxDynamicPort",
			conf: Config{
				Client: &ClientConfig{
					Enabled:        true,
					MaxDynamicPort: math.MaxInt32,
				},
			},
			err: "max_dynamic_port",
		},
		{
			name: "MinMaxDynamicPortSwitched",
			conf: Config{
				Client: &ClientConfig{
					Enabled:        true,
					MinDynamicPort: 5000,
					MaxDynamicPort: 4000,
				},
			},
			err: "and max",
		},
		{
			name: "DynamicPortOk",
			conf: Config{
				DataDir: "/tmp",
				Client: &ClientConfig{
					Enabled:        true,
					MinDynamicPort: 4000,
					MaxDynamicPort: 5000,
				},
			},
		},
		{
			name: "BadReservedPorts",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
					Reserved: &Resources{
						ReservedPorts: "3-2147483647",
					},
				},
			},
			err: `reserved.reserved_ports "3-2147483647" invalid: port must be < 65536 but found 2147483647`,
		},
		{
			name: "BadHostNetworkReservedPorts",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
					HostNetworks: []*structs.ClientHostNetworkConfig{
						&structs.ClientHostNetworkConfig{
							Name:          "test",
							ReservedPorts: "3-2147483647",
						},
					},
				},
			},
			err: `host_network["test"].reserved_ports "3-2147483647" invalid: port must be < 65536 but found 2147483647`,
		},
		{
			name: "BadArtifact",
			conf: Config{
				Client: &ClientConfig{
					Enabled: true,
					Artifact: &config.ArtifactConfig{
						HTTPReadTimeout: pointer.Of("-10m"),
					},
				},
			},
			err: "client.artifact block invalid: http_read_timeout must be > 0",
		},
		{
			name: "BadHostVolumeConfig",
			conf: Config{
				DataDir: "/tmp",
				Client: &ClientConfig{
					Enabled: true,
					HostVolumes: []*structs.ClientHostVolumeConfig{
						{
							Name:     "test",
							ReadOnly: true,
						},
						{
							Name:     "test",
							ReadOnly: true,
							Path:     "/random/path",
						},
					},
				},
			},
			err: "Missing path in host_volume config",
		},
		{
			name: "ValidHostVolumeConfig",
			conf: Config{
				DataDir: "/tmp",
				Client: &ClientConfig{
					Enabled: true,
					HostVolumes: []*structs.ClientHostVolumeConfig{
						{
							Name:     "test",
							ReadOnly: true,
							Path:     "/random/path1",
						},
						{
							Name:     "test",
							ReadOnly: true,
							Path:     "/random/path2",
						},
					},
				},
			},
		},
		{
			name: "BadOIDCIssuer",
			conf: Config{
				DataDir: "/tmp",
				Server: &ServerConfig{
					Enabled:    true,
					OIDCIssuer: ":/example.com",
				},
			},
			err: "missing protocol scheme",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mui := cli.NewMockUi()
			cmd := &Command{Ui: mui}
			config := DefaultConfig().Merge(&tc.conf)
			result := cmd.IsValidConfig(config, DefaultConfig())
			if tc.err == "" {
				// No error expected
				assert.True(t, result, mui.ErrorWriter.String())
				return
			}

			// Error expected
			assert.False(t, result)
			require.Contains(t, mui.ErrorWriter.String(), tc.err)
			t.Logf("%s returned: %s", tc.name, mui.ErrorWriter.String())
		})
	}
}

func TestCommand_readConfig(t *testing.T) {
	// Don't run in parallel since this test modifies environment variables.

	configFiles := map[string]string{
		"base.hcl": `
data_dir = "/tmp/nomad"
region   = "global"

server {
  enabled = true
}

client {
  enabled = true
}
`,
		"vault.hcl": `
data_dir = "/tmp/nomad"
region   = "global"

server {
  enabled = true
}

client {
  enabled = true
}

vault {
  namespace = "ns-from-config"
}
`,
	}

	configDir := t.TempDir()
	for k, v := range configFiles {
		err := os.WriteFile(path.Join(configDir, k), []byte(v), 0644)
		must.NoError(t, err)
	}

	testCases := []struct {
		name    string
		args    []string
		env     map[string]string
		checkFn func(*testing.T, *Config)
	}{
		{
			name: "namespace from env var",
			args: []string{
				"-config", path.Join(configDir, "base.hcl"),
			},
			env: map[string]string{
				"VAULT_NAMESPACE": "ns-from-env",
			},
			checkFn: func(t *testing.T, c *Config) {
				must.Eq(t, "ns-from-env", c.Vaults[0].Namespace)
			},
		},
		{
			name: "namespace from config takes precedence over env var",
			args: []string{
				"-config", path.Join(configDir, "vault.hcl"),
			},
			env: map[string]string{
				"VAULT_NAMESPACE": "ns-from-env",
			},
			checkFn: func(t *testing.T, c *Config) {
				must.Eq(t, "ns-from-config", c.Vaults[0].Namespace)
			},
		},
		{
			name: "namespace from flag takes precedence over env var and config",
			args: []string{
				"-config", path.Join(configDir, "vault.hcl"),
				"-vault-namespace", "ns-from-cli",
			},
			env: map[string]string{
				"VAULT_NAMESPACE": "ns-from-env",
			},
			checkFn: func(t *testing.T, c *Config) {
				must.Eq(t, "ns-from-cli", c.Vaults[0].Namespace)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			defer func() {
				// Print command stderr in case of a failed test case to help
				// with debugging.
				if t.Failed() {
					t.Log(ui.ErrorWriter.String())
				}
			}()

			cmd := &Command{
				Ui:   ui,
				args: tc.args,
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got := cmd.readConfig()
			must.NotNil(t, got)
			tc.checkFn(t, got)
		})
	}
}

func Test_setupLoggers_logFile(t *testing.T) {

	// Generate a mock UI and temporary log file location to write to.
	mockUI := cli.NewMockUi()
	logFile := filepath.Join(t.TempDir(), "nomad.log")

	// The initial configuration contains an invalid log level parameter.
	cfg := &Config{
		LogFile:  logFile,
		LogLevel: "warning",
	}

	// Generate the loggers and ensure the correct error is generated.
	gatedWriter, writer := SetupLoggers(mockUI, cfg)
	must.Nil(t, gatedWriter)
	must.Nil(t, writer)
	must.StrContains(t, mockUI.ErrorWriter.String(), "Invalid log level: WARNING")

	mockUI.ErrorWriter.Reset()
	mockUI.OutputWriter.Reset()

	// Update the log level, so that it is a valid option and set up the
	// loggers again.
	cfg.LogLevel = "warn"
	gatedWriter, writer = SetupLoggers(mockUI, cfg)
	must.NotNil(t, gatedWriter)
	must.NotNil(t, writer)

	// Build the logger as the command does.
	testLogger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   "agent",
		Level:  hclog.LevelFromString(cfg.LogLevel),
		Output: writer,
	})

	// Flush the log gate and write messages at all levels.
	gatedWriter.Flush()
	testLogger.Error("error log entry")
	testLogger.Warn("warn log entry")
	testLogger.Info("info log entry")
	testLogger.Debug("debug log entry")
	testLogger.Trace("trace log entry")

	// Read the file out and ensure it only contains log entries which match
	// our desired level.
	fileContents, err := os.ReadFile(logFile)
	must.NoError(t, err)

	fileContentsStr := string(fileContents)
	must.StrContains(t, fileContentsStr, "error log entry")
	must.StrContains(t, fileContentsStr, "warn log entry")
	must.StrNotContains(t, fileContentsStr, "info log entry")
	must.StrNotContains(t, fileContentsStr, "debug log entry")
	must.StrNotContains(t, fileContentsStr, "trace log entry")
}
