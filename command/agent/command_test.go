package agent

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/version"
)

func TestCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &Command{}
}

func TestCommand_Args(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

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
			[]string{"-server"},
			"Must specify \"data_dir\" config option or \"data-dir\" CLI flag",
		},
		{
			[]string{"-client", "-alloc-dir="},
			"Must specify the state, alloc dir, and plugin dir if data-dir is omitted.",
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
	tmpDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	tcases := []string{
		"foo..invalid",
		".invalid",
		"invalid.",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err = ioutil.WriteFile(configFile, []byte(`client{
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

func TestCommand_NullCharInDatacenter(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	tcases := []string{
		"char-\\000-in-the-middle",
		"ends-with-\\000",
		"\\000-at-the-beginning",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err = ioutil.WriteFile(configFile, []byte(`
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
		exp := "Datacenter contains invalid characters"
		if !strings.Contains(out, exp) {
			t.Fatalf("expect to find %q\n\n%s", exp, out)
		}
	}
}

func TestCommand_NullCharInRegion(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	tcases := []string{
		"char-\\000-in-the-middle",
		"ends-with-\\000",
		"\\000-at-the-beginning",
	}
	for _, tc := range tcases {
		configFile := filepath.Join(tmpDir, "conf1.hcl")
		err = ioutil.WriteFile(configFile, []byte(`
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mui := cli.NewMockUi()
			cmd := &Command{Ui: mui}
			config := DefaultConfig().Merge(&tc.conf)
			result := cmd.isValidConfig(config, DefaultConfig())
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
