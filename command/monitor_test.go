package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
)

func TestMonitor_Update_Eval(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
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

	// Evals trigerred by nodes log
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
	t.Parallel()
	ui := new(cli.MockUi)
	mon := newMonitor(ui, nil, fullId)

	// New allocations write new logs
	state := &evalState{
		allocs: map[string]*allocState{
			"alloc1": &allocState{
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
			"alloc1": &allocState{
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
	t.Parallel()
	ui := new(cli.MockUi)
	mon := newMonitor(ui, nil, fullId)

	// New allocs with a create index lower than the
	// eval create index are logged as modifications
	state := &evalState{
		index: 2,
		allocs: map[string]*allocState{
			"alloc3": &allocState{
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
	t.Parallel()
	srv, client, _ := testServer(t, false, nil)
	defer srv.Shutdown()

	// Create the monitor
	ui := new(cli.MockUi)
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
		code = mon.monitor(resp.EvalID, false)
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

func TestMonitor_MonitorWithPrefix(t *testing.T) {
	t.Parallel()
	srv, client, _ := testServer(t, false, nil)
	defer srv.Shutdown()

	// Create the monitor
	ui := new(cli.MockUi)
	mon := newMonitor(ui, client, shortId)

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
		code = mon.monitor(resp.EvalID[:13], true)
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
	if !strings.Contains(out, resp.EvalID[:8]) {
		t.Fatalf("missing eval\n\n%s", out)
	}
	if strings.Contains(out, resp.EvalID) {
		t.Fatalf("expected truncated eval id, got: %s", out)
	}
	if !strings.Contains(out, "finished with status") {
		t.Fatalf("missing final status\n\n%s", out)
	}

	// Fail on identifier with too few characters
	code = mon.monitor(resp.EvalID[:1], true)
	if code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "must contain at least two characters.") {
		t.Fatalf("expected too few characters error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	code = mon.monitor(resp.EvalID[:3], true)
	if code != 2 {
		t.Fatalf("expect exit 2, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Monitoring evaluation") {
		t.Fatalf("expected evaluation monitoring output, got: %s", out)
	}

}

func TestMonitor_DumpAllocStatus(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)

	// Create an allocation and dump its status to the UI
	alloc := &api.Allocation{
		ID:           "87654321-abcd-efab-cdef-123456789abc",
		TaskGroup:    "group1",
		ClientStatus: structs.AllocClientStatusRunning,
		Metrics: &api.AllocationMetric{
			NodesEvaluated: 10,
			NodesFiltered:  5,
			NodesExhausted: 1,
			DimensionExhausted: map[string]int{
				"cpu": 1,
			},
			ConstraintFiltered: map[string]int{
				"$attr.kernel.name = linux": 1,
			},
			ClassExhausted: map[string]int{
				"web-large": 1,
			},
		},
	}
	dumpAllocStatus(ui, alloc, fullId)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, structs.AllocClientStatusRunning) {
		t.Fatalf("missing status\n\n%s", out)
	}
	if !strings.Contains(out, "5/10") {
		t.Fatalf("missing filter stats\n\n%s", out)
	}
	if !strings.Contains(
		out, `Constraint "$attr.kernel.name = linux" filtered 1 nodes`) {
		t.Fatalf("missing constraint\n\n%s", out)
	}
	if !strings.Contains(out, "Resources exhausted on 1 nodes") {
		t.Fatalf("missing resource exhaustion\n\n%s", out)
	}
	if !strings.Contains(out, `Class "web-large" exhausted on 1 nodes`) {
		t.Fatalf("missing class exhaustion\n\n%s", out)
	}
	if !strings.Contains(out, `Dimension "cpu" exhausted on 1 nodes`) {
		t.Fatalf("missing dimension exhaustion\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// Dumping alloc status with no eligible nodes adds a warning
	alloc.Metrics.NodesEvaluated = 0
	dumpAllocStatus(ui, alloc, shortId)

	// Check the output
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "No nodes were eligible") {
		t.Fatalf("missing eligibility warning\n\n%s", out)
	}
	if strings.Contains(out, "87654321-abcd-efab-cdef-123456789abc") {
		t.Fatalf("expected truncated id, got %s", out)
	}
	if !strings.Contains(out, "87654321") {
		t.Fatalf("expected alloc id, got %s", out)
	}
}
