package command

import (
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"
)

func TestOperator_Autopilot_SetConfig_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperatorAutopilotSetConfigCommand(t *testing.T) {
	t.Parallel()
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := new(cli.MockUi)
	c := &OperatorAutopilotSetCommand{Meta: Meta{Ui: ui}}
	args := []string{
		"-address=" + addr,
		"-cleanup-dead-servers=false",
		"-max-trailing-logs=99",
		"-last-contact-threshold=123ms",
		"-server-stabilization-time=123ms",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "Configuration updated") {
		t.Fatalf("bad: %s", output)
	}

	client, err := c.Client()
	if err != nil {
		t.Fatal(err)
	}

	conf, _, err := client.Operator().AutopilotGetConfiguration(nil)
	if err != nil {
		t.Fatal(err)
	}

	if conf.CleanupDeadServers {
		t.Fatalf("bad: %#v", conf)
	}
	if conf.MaxTrailingLogs != 99 {
		t.Fatalf("bad: %#v", conf)
	}
	if conf.LastContactThreshold != 123*time.Millisecond {
		t.Fatalf("bad: %#v", conf)
	}
	if conf.ServerStabilizationTime != 123*time.Millisecond {
		t.Fatalf("bad: %#v", conf)
	}
}
