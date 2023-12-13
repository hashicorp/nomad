// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestNodeStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeStatusCommand{}
}

func TestNodeStatusCommand_Self(t *testing.T) {
	ci.Parallel(t)
	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "mynode"
	})
	defer srv.Shutdown()

	ui := cli.NewMockUi()
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
	if !strings.Contains(out, "No allocations placed") {
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
	ci.Parallel(t)
	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "mynode"
	})
	defer srv.Shutdown()

	ui := cli.NewMockUi()
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
	if !strings.Contains(out, "No allocations placed") {
		t.Fatalf("should not dump allocations")
	}

	// Query a single node based on a prefix that is even without the hyphen
	if code := cmd.Run([]string{"-address=" + url, nodeID[:13]}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "mynode") {
		t.Fatalf("expect to find mynode, got: %s", out)
	}
	if !strings.Contains(out, nodeID) {
		t.Fatalf("expected node id %q, got: %s", nodeID, out)
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
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
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

	// Fails on nonexistent node
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
	ui.ErrorWriter.Reset()

	// Fail if -quiet is passed with -verbose
	if code := cmd.Run([]string{"-address=" + url, "-quiet", "-verbose"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "-quiet cannot be used with -verbose or -json") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail if -quiet is passed with -json
	if code := cmd.Run([]string{"-address=" + url, "-quiet", "-json"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "-quiet cannot be used with -verbose or -json") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}
}

func TestNodeStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

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

	ui := cli.NewMockUi()
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	prefix := nodeID[:len(nodeID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(nodeID, res[0])
}

func TestNodeStatusCommand_FormatDrain(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	node := &api.Node{}

	assert.Equal("false", formatDrain(node))

	node.DrainStrategy = &api.DrainStrategy{}
	assert.Equal("true; no deadline", formatDrain(node))

	node.DrainStrategy = &api.DrainStrategy{}
	node.DrainStrategy.Deadline = -1 * time.Second
	assert.Equal("true; force drain", formatDrain(node))

	// formatTime special cases Unix(0, 0), so increment by 1
	node.DrainStrategy = &api.DrainStrategy{}
	node.DrainStrategy.ForceDeadline = time.Unix(1, 0).UTC()
	assert.Equal("true; 1970-01-01T00:00:01Z deadline", formatDrain(node))

	node.DrainStrategy.IgnoreSystemJobs = true
	assert.Equal("true; 1970-01-01T00:00:01Z deadline; ignoring system jobs", formatDrain(node))
}
