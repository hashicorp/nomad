// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeDrainCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeDrainCommand{}
}

func TestNodeDrainCommand_Detach(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	server, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "drain_detach_node"
	})
	defer server.Shutdown()

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

	// Register a job to create an alloc to drain that will block draining
	job := &api.Job{
		ID:          pointer.Of("mock_service"),
		Name:        pointer.Of("mock_service"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name: pointer.Of("mock_group"),
				Tasks: []*api.Task{
					{
						Name:   "mock_task",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "10m",
						},
					},
				},
			},
		},
	}

	_, _, err := client.Jobs().Register(job, nil)
	require.Nil(err)

	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err := client.Nodes().Allocations(nodeID, nil)
		if err != nil {
			return false, err
		}
		return len(allocs) > 0, fmt.Errorf("no allocs")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}
	if code := cmd.Run([]string{"-address=" + url, "-self", "-enable", "-detach"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	expected := "drain strategy set"
	require.Contains(out, expected)

	node, _, err := client.Nodes().Info(nodeID, nil)
	require.Nil(err)
	require.NotNil(node.DrainStrategy)
}

func TestNodeDrainCommand_Monitor(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	server, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "drain_monitor_node"
	})
	defer server.Shutdown()

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
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		nodeID = nodes[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Register a service job to create allocs to drain
	serviceCount := 3
	job := &api.Job{
		ID:          pointer.Of("mock_service"),
		Name:        pointer.Of("mock_service"),
		Datacenters: []string{"dc1"},
		Type:        pointer.Of("service"),
		TaskGroups: []*api.TaskGroup{
			{
				Name:  pointer.Of("mock_group"),
				Count: &serviceCount,
				Migrate: &api.MigrateStrategy{
					MaxParallel:     pointer.Of(1),
					HealthCheck:     pointer.Of("task_states"),
					MinHealthyTime:  pointer.Of(10 * time.Millisecond),
					HealthyDeadline: pointer.Of(5 * time.Minute),
				},
				Tasks: []*api.Task{
					{
						Name:   "mock_task",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "10m",
						},
						Resources: &api.Resources{
							CPU:      pointer.Of(50),
							MemoryMB: pointer.Of(50),
						},
					},
				},
			},
		},
	}

	_, _, err := client.Jobs().Register(job, nil)
	require.Nil(err)

	// Register a system job to ensure it is ignored during draining
	sysjob := &api.Job{
		ID:          pointer.Of("mock_system"),
		Name:        pointer.Of("mock_system"),
		Datacenters: []string{"dc1"},
		Type:        pointer.Of("system"),
		TaskGroups: []*api.TaskGroup{
			{
				Name:  pointer.Of("mock_sysgroup"),
				Count: pointer.Of(1),
				Tasks: []*api.Task{
					{
						Name:   "mock_systask",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": "10m",
						},
						Resources: &api.Resources{
							CPU:      pointer.Of(50),
							MemoryMB: pointer.Of(50),
						},
					},
				},
			},
		},
	}

	_, _, err = client.Jobs().Register(sysjob, nil)
	require.Nil(err)

	var allocs []*api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err = client.Nodes().Allocations(nodeID, nil)
		if err != nil {
			return false, err
		}
		if len(allocs) != serviceCount+1 {
			return false, fmt.Errorf("number of allocs %d != count (%d)", len(allocs), serviceCount+1)
		}
		for _, a := range allocs {
			if a.ClientStatus != "running" {
				return false, fmt.Errorf("alloc %q still not running: %s", a.ID, a.ClientStatus)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	outBuf := bytes.NewBuffer(nil)
	ui := &cli.BasicUi{
		Reader:      bytes.NewReader(nil),
		Writer:      outBuf,
		ErrorWriter: outBuf,
	}
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + url, "-self", "-enable", "-deadline", "1s", "-ignore-system"}
	t.Logf("Running: %v", args)
	require.Zero(cmd.Run(args))

	out := outBuf.String()
	t.Logf("Output:\n%s", out)

	// Unfortunately travis is too slow to reliably see the expected output. The
	// monitor goroutines may start only after some or all the allocs have been
	// migrated.
	if !testutil.IsTravis() {
		require.Contains(out, "Drain complete for node")
		for _, a := range allocs {
			if *a.Job.Type == "system" {
				if strings.Contains(out, a.ID) {
					t.Fatalf("output should not contain system alloc %q", a.ID)
				}
				continue
			}
			require.Contains(out, fmt.Sprintf("Alloc %q marked for migration", a.ID))
			require.Contains(out, fmt.Sprintf("Alloc %q draining", a.ID))
		}

		expected := fmt.Sprintf("All allocations on node %q have stopped\n", nodeID)
		if !strings.HasSuffix(out, expected) {
			t.Fatalf("expected output to end with:\n%s", expected)
		}
	}

	// Test -monitor flag
	outBuf.Reset()
	args = []string{"-address=" + url, "-self", "-monitor", "-ignore-system"}
	t.Logf("Running: %v", args)
	require.Zero(cmd.Run(args))

	out = outBuf.String()
	t.Logf("Output:\n%s", out)
	require.Contains(out, "No drain strategy set")
}

func TestNodeDrainCommand_Monitor_NoDrainStrategy(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	server, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "drain_monitor_node2"
	})
	defer server.Shutdown()

	// Wait for a node to appear
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Test -monitor flag
	outBuf := bytes.NewBuffer(nil)
	ui := &cli.BasicUi{
		Reader:      bytes.NewReader(nil),
		Writer:      outBuf,
		ErrorWriter: outBuf,
	}
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + url, "-self", "-monitor", "-ignore-system"}
	t.Logf("Running: %v", args)
	if code := cmd.Run(args); code != 0 {
		t.Fatalf("expected exit 0, got: %d\n%s", code, outBuf.String())
	}

	out := outBuf.String()
	t.Logf("Output:\n%s", out)

	require.Contains(out, "No drain strategy set")
}

func TestNodeDrainCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "-enable", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error toggling") {
		t.Fatalf("expected failed toggle error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on nonexistent node
	if code := cmd.Run([]string{"-address=" + url, "-enable", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No node(s) with prefix or id") {
		t.Fatalf("expected not exist error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if both enable and disable specified
	if code := cmd.Run([]string{"-enable", "-disable", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if neither enable or disable specified
	if code := cmd.Run([]string{"12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	if code := cmd.Run([]string{"-address=" + url, "-enable", "1"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "must contain at least two characters.") {
		t.Fatalf("expected too few characters error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	if code := cmd.Run([]string{"-address=" + url, "-enable", "123"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No node(s) with prefix or id") {
		t.Fatalf("expected not exist error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on disable being used with drain strategy flags
	for _, flag := range []string{"-force", "-no-deadline", "-ignore-system"} {
		if code := cmd.Run([]string{"-address=" + url, "-disable", flag, "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
			t.Fatalf("expected exit 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "combined with flags configuring drain strategy") {
			t.Fatalf("got: %s", out)
		}
		ui.ErrorWriter.Reset()
	}

	// Fail on setting a deadline plus deadline modifying flags
	for _, flag := range []string{"-force", "-no-deadline"} {
		if code := cmd.Run([]string{"-address=" + url, "-enable", "-deadline=10s", flag, "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
			t.Fatalf("expected exit 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "deadline can't be combined with") {
			t.Fatalf("got: %s", out)
		}
		ui.ErrorWriter.Reset()
	}

	// Fail on setting a force and no deadline
	if code := cmd.Run([]string{"-address=" + url, "-enable", "-force", "-no-deadline", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "mutually exclusive") {
		t.Fatalf("got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on setting a bad deadline
	for _, flag := range []string{"-deadline=0s", "-deadline=-1s"} {
		if code := cmd.Run([]string{"-address=" + url, "-enable", flag, "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
			t.Fatalf("expected exit 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "positive") {
			t.Fatalf("got: %s", out)
		}
		ui.ErrorWriter.Reset()
	}
}

func TestNodeDrainCommand_AutocompleteArgs(t *testing.T) {
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
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	prefix := nodeID[:len(nodeID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(nodeID, res[0])
}
