// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestNodeStopCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeStopCommand{}
}

func TestNodeStopCommand_Monitor(t *testing.T) {
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
				Disconnect: &api.DisconnectStrategy{
					LostAfter: pointer.Of(10 * time.Minute),
				},
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

	client.Close()

	cmd := &NodeStopCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + url}
	t.Logf("Running: %v", args)
	require.Zero(cmd.Run(args))

	out := outBuf.String()
	t.Logf("Output:\n%s", out)

	// Test -monitor flag
	outBuf.Reset()
	args = []string{"-address=" + url, "-self"}
	t.Logf("Running: %v", args)
	require.Zero(cmd.Run(args))

	out = outBuf.String()
	t.Logf("Output:\n%s", out)
	require.Contains(out, "No drain strategy set")
}

/* func TestNodeStopCommand_Detach(t *testing.T) {
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
} */
