// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestNodeDrainCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeDrainCommand{}
}

func TestNodeDrainCommand_Detach(t *testing.T) {
	ci.Parallel(t)

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
		ID:          new("mock_service"),
		Name:        new("mock_service"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name: new("mock_group"),
				Tasks: []*api.Task{
					{
						Name:   "mock_task",
						Driver: "mock_driver",
						Config: map[string]any{
							"run_for": "10m",
						},
					},
				},
			},
		},
	}

	_, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

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
	must.StrContains(t, out, expected)

	node, _, err := client.Nodes().Info(nodeID, nil)
	must.NoError(t, err)
	must.NotNil(t, node.DrainStrategy)
}

func TestNodeDrainCommand_DurationAware(t *testing.T) {
	ci.Parallel(t)
	server, client, url := testServer(t, false, nil)
	defer server.Shutdown()
	state := server.Agent.Server().State()
	node := mock.Node()
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// A non-terminal allocation without a real client keeps the drain active
	// long enough to inspect the strategy returned by the API.
	job := mock.BatchJob()
	job.TaskGroups[0].Count = 1
	alloc := mock.MinAllocForJob(job)
	alloc.NodeID = node.ID
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc}))

	for _, tc := range []struct {
		name   string
		flag   string
		buffer time.Duration
		err    string
	}{
		{name: "explicit buffer", flag: "45s", buffer: 45 * time.Second},
		{name: "default buffer"},
		{name: "minimum buffer", flag: "1s", buffer: time.Second},
		{name: "subsecond buffer rejected by server", flag: "500ms", err: "backfill buffer must be at least 1s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err != "" {
				_, err := client.Nodes().UpdateDrain(node.ID, &api.DrainSpec{
					Deadline:       10 * time.Minute,
					DurationAware:  true,
					BackfillBuffer: time.Second,
				}, false, nil)
				must.NoError(t, err)
			}
			ui := cli.NewMockUi()
			cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}
			args := []string{"-address=" + url, "-enable", "-detach", "-duration-aware", "-deadline=10m"}
			if tc.flag != "" {
				args = append(args, "-backfill-buffer="+tc.flag)
			}
			args = append(args, node.ID)
			code := cmd.Run(args)
			if tc.err != "" {
				must.Eq(t, 1, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.err)
			} else {
				must.Zero(t, code, must.Sprint(ui.ErrorWriter.String()))
				must.StrContains(t, ui.OutputWriter.String(), "drain strategy set")
			}
			out, _, err := client.Nodes().Info(node.ID, nil)
			must.NoError(t, err)
			must.NotNil(t, out.DrainStrategy)
			must.True(t, out.DrainStrategy.DurationAware)
			must.Eq(t, 10*time.Minute, out.DrainStrategy.Deadline)
			if tc.err == "" {
				must.Eq(t, tc.buffer, out.DrainStrategy.BackfillBuffer)
			} else {
				// Rejection must preserve the previously accepted strategy.
				must.Eq(t, time.Second, out.DrainStrategy.BackfillBuffer)
			}
			stored, err := state.NodeByID(nil, node.ID)
			must.NoError(t, err)
			must.True(t, stored.DrainStrategy.DurationAware)
			must.Eq(t, out.DrainStrategy.BackfillBuffer, stored.DrainStrategy.BackfillBuffer)
		})
	}
}

func TestNodeDrainCommand_DurationAwareValidation(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name string
		args []string
		err  string
	}{
		{
			name: "force drain",
			args: []string{"-enable", "-duration-aware", "-force"},
			err:  "-duration-aware requires -enable and a positive deadline",
		},
		{
			name: "no deadline",
			args: []string{"-enable", "-duration-aware", "-no-deadline"},
			err:  "-duration-aware requires -enable and a positive deadline",
		},
		{
			name: "duration-aware on disable",
			args: []string{"-disable", "-duration-aware"},
			err:  "-disable can't be combined with flags configuring drain strategy",
		},
		{
			name: "buffer on disable",
			args: []string{"-disable", "-backfill-buffer=30s"},
			err:  "-disable can't be combined with flags configuring drain strategy",
		},
		{
			name: "zero deadline",
			args: []string{"-enable", "-duration-aware", "-deadline=0s"},
			err:  "A positive drain duration must be given",
		},
		{
			name: "negative deadline",
			args: []string{"-enable", "-duration-aware", "-deadline=-1s"},
			err:  "A positive drain duration must be given",
		},
		{
			name: "negative buffer",
			args: []string{"-enable", "-duration-aware", "-backfill-buffer=-1s"},
			err:  "-backfill-buffer must be non-negative and requires -duration-aware",
		},
		{
			name: "buffer without duration awareness",
			args: []string{"-enable", "-backfill-buffer=30s"},
			err:  "-backfill-buffer must be non-negative and requires -duration-aware",
		},
		{
			name: "malformed buffer",
			args: []string{"-enable", "-duration-aware", "-backfill-buffer=invalid"},
			err:  "invalid value",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}
			args := append([]string{"-address=http://127.0.0.1:1"}, tc.args...)
			args = append(args, "12345678-abcd-efab-cdef-123456789abc")
			must.Eq(t, 1, cmd.Run(args))
			must.StrContains(t, ui.ErrorWriter.String(), tc.err)
		})
	}
}

func TestNodeDrainCommand_Monitor(t *testing.T) {
	ci.Parallel(t)

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
		ID:          new("mock_service"),
		Name:        new("mock_service"),
		Datacenters: []string{"dc1"},
		Type:        new("service"),
		TaskGroups: []*api.TaskGroup{
			{
				Name:  new("mock_group"),
				Count: &serviceCount,
				Migrate: &api.MigrateStrategy{
					MaxParallel:     new(1),
					HealthCheck:     new("task_states"),
					MinHealthyTime:  new(10 * time.Millisecond),
					HealthyDeadline: new(5 * time.Minute),
				},
				Tasks: []*api.Task{
					{
						Name:   "mock_task",
						Driver: "mock_driver",
						Config: map[string]any{
							"run_for": "10m",
						},
						Resources: &api.Resources{
							CPU:      new(50),
							MemoryMB: new(50),
						},
					},
				},
			},
		},
	}

	_, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	// Register a system job to ensure it is ignored during draining
	sysjob := &api.Job{
		ID:          new("mock_system"),
		Name:        new("mock_system"),
		Datacenters: []string{"dc1"},
		Type:        new("system"),
		TaskGroups: []*api.TaskGroup{
			{
				Name:  new("mock_sysgroup"),
				Count: new(1),
				Tasks: []*api.Task{
					{
						Name:   "mock_systask",
						Driver: "mock_driver",
						Config: map[string]any{
							"run_for": "10m",
						},
						Resources: &api.Resources{
							CPU:      new(50),
							MemoryMB: new(50),
						},
					},
				},
			},
		},
	}

	_, _, err = client.Jobs().Register(sysjob, nil)
	must.NoError(t, err)

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
	must.Zero(t, cmd.Run(args))

	out := outBuf.String()
	t.Logf("Output:\n%s", out)

	// Test -monitor flag
	outBuf.Reset()
	args = []string{"-address=" + url, "-self", "-monitor", "-ignore-system"}
	t.Logf("Running: %v", args)
	must.Zero(t, cmd.Run(args))

	out = outBuf.String()
	t.Logf("Output:\n%s", out)
	must.StrContains(t, out, "No drain strategy set")
}

func TestNodeDrainCommand_Monitor_NoDrainStrategy(t *testing.T) {
	ci.Parallel(t)

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

	must.StrContains(t, out, "No drain strategy set")
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
	must.Len(t, 1, res)
	must.Eq(t, nodeID, res[0])
}
