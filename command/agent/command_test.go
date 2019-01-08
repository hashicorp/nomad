package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/version"
	"github.com/mitchellh/cli"
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
			"Must specify data directory",
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
		ui := new(cli.MockUi)
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
		ui := new(cli.MockUi)
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
