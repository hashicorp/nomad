package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
)

func TestAllocStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &AllocStatusCommand{}
}

func TestAllocStatusCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying allocation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}

func TestAllocStatus_DumpAllocStatus(t *testing.T) {
	ui := new(cli.MockUi)

	// Create an allocation and dump its status to the UI
	alloc := &api.Allocation{
		ID:           "alloc1",
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
	dumpAllocStatus(ui, alloc)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "alloc1") {
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
}
