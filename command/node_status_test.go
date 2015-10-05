package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/mitchellh/cli"
	"github.com/hashicorp/nomad/testutil"
)

func TestNodeStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &NodeStatusCommand{}
}

func TestNodeStatusCommand_Run(t *testing.T) {
	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, func(c *testutil.TestServerConfig) {
		c.DevMode = true
		c.NodeName = "mynode"
	})
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui}}

	// Wait for a node to appear
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		nodeID = nodes[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Query all node statuses
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query a single node
	if code := cmd.Run([]string{"-address=" + url, nodeID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	if !strings.Contains(out, "Allocations") {
		t.Fatalf("expected allocations, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Query single node in short view
	if code := cmd.Run([]string{"-address=" + url, "-short", nodeID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	if strings.Contains(out, "Allocations") {
		t.Fatalf("should not dump allocations")
	}
}

func TestNodeStatusCommand_Fails(t *testing.T) {
	srv, _, url := testServer(t, nil)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying node status") {
		t.Fatalf("expected failed query error, got: %s", out)
	}

	// Fails on non-existent node
	if code := cmd.Run([]string{"-address=" + url, "nope"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "not found") {
		t.Fatalf("expected not found error, got: %s", out)
	}
}
