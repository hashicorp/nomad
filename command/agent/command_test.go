package agent

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestCommand_Implements(t *testing.T) {
	var _ cli.Command = &Command{}
}

func TestCommand_Args(t *testing.T) {
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
			"Must specify data directory",
		},
		{
			[]string{"-data-dir=" + tmpDir, "-bootstrap-expect=1"},
			"Bootstrap requires server mode to be enabled",
		},
		{
			[]string{"-data-dir=" + tmpDir, "-server", "-bootstrap-expect=1"},
			"WARNING: Bootstrap mode enabled!",
		},
	}
	for _, tc := range tcases {
		// Make a new command. We pre-emptively close the shutdownCh
		// so that the command exits immediately instead of blocking.
		ui := new(cli.MockUi)
		shutdownCh := make(chan struct{})
		close(shutdownCh)
		cmd := &Command{
			Ui:         ui,
			ShutdownCh: shutdownCh,
		}

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
