package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
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
			"Must specify both the state and alloc dir if data-dir is omitted.",
		},
	}
	for _, tc := range tcases {
		// Make a new command. We pre-emptively close the shutdownCh
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

// TODO Why is this failing
func TestRetryJoin(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent(t.Name(), nil)
	defer agent.Shutdown()

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	cmd := &Command{
		Version:    version.GetVersion(),
		ShutdownCh: shutdownCh,
		Ui: &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		},
	}

	serfAddr := agent.Config.normalizedAddrs.Serf

	args := []string{
		"-dev",
		"-node", "foo",
		"-retry-join", serfAddr,
		"-retry-interval", "1s",
	}

	go func() {
		if code := cmd.Run(args); code != 0 {
			t.Logf("bad: %d", code)
		}
		close(doneCh)
	}()

	testutil.WaitForResult(func() (bool, error) {
		mem := agent.server.Members()
		if len(mem) != 2 {
			return false, fmt.Errorf("bad :%#v", mem)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf(err.Error())
	})
}
