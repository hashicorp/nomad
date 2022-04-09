package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestAgentCheckCommand_ServerHealth(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentCheckCommand{Meta: Meta{Ui: ui}}
	address := fmt.Sprintf("-address=%s", url)

	code := cmd.Run([]string{address})
	if code != HealthPass {
		t.Fatalf("expected exit: %v, actual: %d", HealthPass, code)
	}

	minPeers := fmt.Sprintf("-min-peers=%v", 3)
	code = cmd.Run([]string{address, minPeers})
	if code != HealthCritical {
		t.Fatalf("expected exitcode: %v, actual: %v", HealthCritical, code)
	}
}
