package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
)

func TestMonitor_Update(t *testing.T) {
	ui := new(cli.MockUi)
	mon := newMonitor(ui, nil)

	// Basic eval updates work
	eval := &api.Evaluation{
		Status:      "pending",
		NodeID:      "node1",
		Wait:        10 * time.Second,
		CreateIndex: 2,
	}
	mon.update(eval, nil)

	// Logs were output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "pending") {
		t.Fatalf("missing status\n\n%s", out)
	}
	if !strings.Contains(out, "node1") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "10s") {
		t.Fatalf("missing eval wait\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// No logs sent if no state update
	mon.update(eval, nil)
	if out := ui.OutputWriter.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}

	// Updates cause more logs to output
	eval.Status = "complete"
	mon.update(eval, nil)
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "complete") {
		t.Fatalf("missing status\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// New allocations write new logs
	allocs := []*api.AllocationListStub{
		&api.AllocationListStub{
			ID:            "alloc1",
			TaskGroup:     "group1",
			NodeID:        "node1",
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusPending,
			CreateIndex:   3,
		},
	}
	mon.update(eval, allocs)

	// Logs were output
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "alloc1") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group1") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "node1") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("missing created\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// No change yields no logs
	mon.update(eval, allocs)
	if out := ui.OutputWriter.String(); out != "" {
		t.Fatalf("expected no output\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// Updates cause more log lines
	allocs[0].ClientStatus = "running"
	mon.update(eval, allocs)
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "alloc1") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Fatalf("missing old status\n\n%s", out)
	}
	if !strings.Contains(out, "running") {
		t.Fatalf("missing new status\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// New allocs with desired status failed warns
	allocs = append(allocs, &api.AllocationListStub{
		ID:                 "alloc2",
		TaskGroup:          "group2",
		DesiredStatus:      structs.AllocDesiredStatusFailed,
		DesiredDescription: "something failed",
		CreateIndex:        4,
	})
	mon.update(eval, allocs)

	// Scheduling failure was logged
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "group2") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "Scheduling error") {
		t.Fatalf("missing failure\n\n%s", out)
	}
	if !strings.Contains(out, "something failed") {
		t.Fatalf("missing reason\n\n%s", out)
	}
	ui.OutputWriter.Reset()

	// New allocs with a create index lower than the
	// eval create index are logged as modifications
	allocs = append(allocs, &api.AllocationListStub{
		ID:          "alloc3",
		NodeID:      "node1",
		TaskGroup:   "group2",
		CreateIndex: 1,
	})
	mon.update(eval, allocs)

	// Modification was logged
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "alloc3") {
		t.Fatalf("missing alloc\n\n%s", out)
	}
	if !strings.Contains(out, "group2") {
		t.Fatalf("missing group\n\n%s", out)
	}
	if !strings.Contains(out, "node1") {
		t.Fatalf("missing node\n\n%s", out)
	}
	if !strings.Contains(out, "modified") {
		t.Fatalf("missing modification\n\n%s", out)
	}
}

func TestMonitor_Monitor(t *testing.T) {
	srv, client, _ := testServer(t, nil)
	defer srv.Stop()

	// Create the monitor
	ui := new(cli.MockUi)
	mon := newMonitor(ui, client)

	// Submit a job - this creates a new evaluation we can monitor
	job := testJob("job1")
	evalID, _, err := client.Jobs().Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Start monitoring the eval
	var code int
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		code = mon.monitor(evalID)
	}()

	// Wait for completion
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("eval monitor took too long")
	}

	// Check the return code
	if code != 0 {
		t.Fatalf("expect exit 0, got: %d", code)
	}

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, evalID) {
		t.Fatalf("missing eval\n\n%s", out)
	}
	if !strings.Contains(out, "finished with status") {
		t.Fatalf("missing final status\n\n%s", out)
	}
}
