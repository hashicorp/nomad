// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestMonitor_Update_Eval(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	mon := newMonitor(ui, nil, fullId)

	// Evals triggered by jobs log
	state := &evalState{
		status: structs.EvalStatusPending,
		job:    "job1",
	}
	mon.update(state)

	out := ui.OutputWriter.String()
	if !strings.Contains(out, "job1") {
		t.Fatalf("missing job\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// Evals triggered by nodes log
	state = &evalState{
		status: structs.EvalStatusPending,
		node:   "12345678-abcd-efab-cdef-123456789abc",
	}
	mon.update(state)

	out = ui.OutputWriter.String()
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}

	// Transition to pending should not be logged
	if strings.Contains(out, structs.EvalStatusPending) {
		t.Fatalf("should skip status\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// No logs sent if no update
	mon.update(state)
	if out := ui.OutputWriter.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}

	// Status change sends more logs
	state = &evalState{
		status: structs.EvalStatusComplete,
		node:   "12345678-abcd-efab-cdef-123456789abc",
	}
	mon.update(state)
	out = ui.OutputWriter.String()
	if !strings.Contains(out, structs.EvalStatusComplete) {
		t.Fatalf("missing status\n\n%s", out)
	}
}

func TestMonitor_Update_Allocs(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	mon := newMonitor(ui, nil, fullId)

	// New allocations write new logs
	state := &evalState{
		allocs: map[string]*allocState{
			"alloc1": {
				id:      "87654321-abcd-efab-cdef-123456789abc",
				group:   "group1",
				node:    "12345678-abcd-efab-cdef-123456789abc",
				desired: structs.AllocDesiredStatusRun,
				client:  structs.AllocClientStatusPending,
				index:   1,
			},
		},
	}
	mon.update(state)

	// Logs were output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group1") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("missing created\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// No change yields no logs
	mon.update(state)
	if out := ui.OutputWriter.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// Alloc updates cause more log lines
	state = &evalState{
		allocs: map[string]*allocState{
			"alloc1": {
				id:      "87654321-abcd-efab-cdef-123456789abc",
				group:   "group1",
				node:    "12345678-abcd-efab-cdef-123456789abc",
				desired: structs.AllocDesiredStatusRun,
				client:  structs.AllocClientStatusRunning,
				index:   2,
			},
		},
	}
	mon.update(state)

	// Updates were logged
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Fatalf("missing old status\n\n%s", out)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("missing new status\n\n%s", out)
	}
}

func TestMonitor_Update_AllocModification(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	mon := newMonitor(ui, nil, fullId)

	// New allocs with a create index lower than the
	// eval create index are logged as modifications
	state := &evalState{
		index: 2,
		allocs: map[string]*allocState{
			"alloc3": {
				id:    "87654321-abcd-bafe-cdef-123456789abc",
				node:  "12345678-abcd-efab-cdef-123456789abc",
				group: "group2",
				index: 1,
			},
		},
	}
	mon.update(state)

	// Modification was logged
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "87654321-abcd-bafe-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group2") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "12345678-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "modified") {
		t.Fatalf("missing modification\n\n%s", out)
	}
}

func TestMonitor_Monitor(t *testing.T) {
	ci.Parallel(t)
	srv, client, _ := testServer(t, false, nil)
	defer srv.Shutdown()

	// Create the monitor
	ui := cli.NewMockUi()
	mon := newMonitor(ui, client, fullId)

	// Submit a job - this creates a new evaluation we can monitor
	job := testJob("job1")
	resp, _, err := client.Jobs().Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Start monitoring the eval
	var code int
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		code = mon.monitor(resp.EvalID)
	}()

	// Wait for completion
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("eval monitor took too long")
	}

	// Check the return code. We should get exit code 2 as there
	// would be a scheduling problem on the test server (no clients).
	if code != 2 {
		t.Fatalf("expect exit 2, got: %d", code)
	}

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, resp.EvalID) {
		t.Fatalf("missing eval\n\n%s", out)
	}
	if !strings.Contains(out, "finished with status") {
		t.Fatalf("missing final status\n\n%s", out)
	}
}

func TestMonitor_formatAllocMetric(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		Name     string
		Metrics  *api.AllocationMetric
		Expected string
	}{
		{
			Name: "display all possible scores",
			Metrics: &api.AllocationMetric{
				NodesEvaluated: 3,
				NodesInPool:    3,
				ScoreMetaData: []*api.NodeScoreMeta{
					{
						NodeID: "node-1",
						Scores: map[string]float64{
							"score-1": 1,
							"score-2": 2,
						},
						NormScore: 1,
					},
					{
						NodeID: "node-2",
						Scores: map[string]float64{
							"score-1": 1,
							"score-3": 3,
						},
						NormScore: 2,
					},
					{
						NodeID: "node-3",
						Scores: map[string]float64{
							"score-4": 4,
						},
						NormScore: 3,
					},
				},
			},
			Expected: `
Node    score-1  score-2  score-3  score-4  final score
node-1  1        2        0        0        1
node-2  1        0        3        0        2
node-3  0        0        0        4        3
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			got := formatAllocMetrics(tc.Metrics, true, "")
			require.Equal(t, strings.TrimSpace(tc.Expected), got)
		})
	}
}
