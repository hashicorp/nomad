package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
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

func TestRetryJoin(t *testing.T) {
	dir, agent := makeAgent(t, nil)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	tmpDir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	serfAddr := fmt.Sprintf(
		"%s:%d",
		agent.config.BindAddr,
		agent.config.Ports.Serf)

	args := []string{
		"-server",
		"-data-dir", tmpDir,
		"-node", fmt.Sprintf(`"Node %d"`, getPort()),
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
