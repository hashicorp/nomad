package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
)

func TestNodeStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &NodeStatusCommand{}
}

func TestNodeStatusCommand_Self(t *testing.T) {
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

	// Query self node
	if code := cmd.Run([]string{"-address=" + url, "-self"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	if strings.Contains(out, "Allocations") {
		t.Fatalf("should not dump allocations")
	}
	ui.OutputWriter.Reset()

	// Request full id output
	if code := cmd.Run([]string{"-address=" + url, "-self", "-verbose"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, nodeID) {
		t.Fatalf("expected full node id %q, got: %s", nodeID, out)
	}
	ui.OutputWriter.Reset()
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

	// Query a single node based on prefix
	if code := cmd.Run([]string{"-address=" + url, nodeID[:4]}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	if strings.Contains(out, nodeID) {
		t.Fatalf("expected truncated node id, got: %s", out)
	}
	if !strings.Contains(out, nodeID[:8]) {
		t.Fatalf("expected node id %q, got: %s", nodeID[:8], out)
	}
	ui.OutputWriter.Reset()

	// Request full id output
	if code := cmd.Run([]string{"-address=" + url, "-verbose", nodeID[:4]}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, nodeID) {
		t.Fatalf("expected full node id %q, got: %s", nodeID, out)
	}
	ui.OutputWriter.Reset()

	// Identifiers with uneven length should produce a query result
	if code := cmd.Run([]string{"-address=" + url, nodeID[:3]}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
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
	ui.ErrorWriter.Reset()

	// Fails on non-existent node
	if code := cmd.Run([]string{"-address=" + url, "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No node(s) with prefix") {
		t.Fatalf("expected not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	if code := cmd.Run([]string{"-address=" + url, "1"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "must contain at least two characters.") {
		t.Fatalf("expected too few characters error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Failed on both -json and -t options are specified
	if code := cmd.Run([]string{"-address=" + url, "-json", "-t", "{{.ID}}"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Both json and template formatting are not allowed") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}
}
