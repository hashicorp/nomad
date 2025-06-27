// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestEvalStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &EvalStatusCommand{}
}

func TestEvalStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EvalStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on eval lookup failure
	if code := cmd.Run([]string{"-address=" + url, "3E55C771-76FC-423B-BCED-3E5314F433B1"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No evaluation(s) with prefix or id") {
		t.Fatalf("expect not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying evaluation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Failed on both -json and -t options are specified
	if code := cmd.Run([]string{"-address=" + url, "-json", "-t", "{{.ID}}",
		"12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Both json and template formatting are not allowed") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}

}

func TestEvalStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EvalStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake eval
	state := srv.Agent.Server().State()
	e := mock.Eval()
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{e}))

	prefix := e.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.SliceLen(t, 1, res)
	must.Eq(t, e.ID, res[0])
}

func TestEvalStatusCommand_Format(t *testing.T) {
	now := time.Now().UTC()
	ui := cli.NewMockUi()
	cmd := &EvalStatusCommand{Meta: Meta{Ui: ui}}

	eval := &api.Evaluation{
		ID:                uuid.Generate(),
		Priority:          50,
		Type:              api.JobTypeService,
		TriggeredBy:       structs.EvalTriggerAllocStop,
		Namespace:         api.DefaultNamespace,
		JobID:             "example",
		JobModifyIndex:    0,
		DeploymentID:      uuid.Generate(),
		Status:            api.EvalStatusComplete,
		StatusDescription: "complete",
		NextEval:          "",
		PreviousEval:      uuid.Generate(),
		BlockedEval:       uuid.Generate(),
		RelatedEvals: []*api.EvaluationStub{{
			ID:                uuid.Generate(),
			Priority:          50,
			Type:              "service",
			TriggeredBy:       "queued-allocs",
			Namespace:         api.DefaultNamespace,
			JobID:             "example",
			DeploymentID:      "",
			Status:            "pending",
			StatusDescription: "",
			WaitUntil:         time.Time{},
			NextEval:          "",
			PreviousEval:      uuid.Generate(),
			BlockedEval:       "",
			CreateIndex:       0,
			ModifyIndex:       0,
			CreateTime:        0,
			ModifyTime:        0,
		}},
		FailedTGAllocs: map[string]*api.AllocationMetric{"web": {
			NodesEvaluated:     6,
			NodesFiltered:      4,
			NodesInPool:        10,
			NodesAvailable:     map[string]int{},
			ClassFiltered:      map[string]int{},
			ConstraintFiltered: map[string]int{"${attr.kernel.name} = linux": 2},
			NodesExhausted:     2,
			ClassExhausted:     map[string]int{},
			DimensionExhausted: map[string]int{"memory": 2},
			QuotaExhausted:     []string{},
			ResourcesExhausted: map[string]*api.Resources{"web": {
				Cores: pointer.Of(3),
			}},
			Scores:            map[string]float64{},
			AllocationTime:    0,
			CoalescedFailures: 0,
			ScoreMetaData:     []*api.NodeScoreMeta{},
		}},
		ClassEligibility:     map[string]bool{},
		EscapedComputedClass: true,
		QuotaLimitReached:    "",
		QueuedAllocations:    map[string]int{},
		SnapshotIndex:        1001,
		CreateIndex:          999,
		ModifyIndex:          1003,
		CreateTime:           now.UnixNano(),
		ModifyTime:           now.Add(time.Second).UnixNano(),
	}

	placed := []*api.AllocationListStub{
		{
			ID:            uuid.Generate(),
			NodeID:        uuid.Generate(),
			TaskGroup:     "web",
			DesiredStatus: "run",
			JobVersion:    2,
			ClientStatus:  "running",
			CreateTime:    now.Add(-10 * time.Second).UnixNano(),
			ModifyTime:    now.Add(-2 * time.Second).UnixNano(),
		},
		{
			ID:            uuid.Generate(),
			NodeID:        uuid.Generate(),
			TaskGroup:     "web",
			JobVersion:    2,
			DesiredStatus: "run",
			ClientStatus:  "pending",
			CreateTime:    now.Add(-3 * time.Second).UnixNano(),
			ModifyTime:    now.Add(-1 * time.Second).UnixNano(),
		},
		{
			ID:            uuid.Generate(),
			NodeID:        uuid.Generate(),
			TaskGroup:     "web",
			JobVersion:    2,
			DesiredStatus: "run",
			ClientStatus:  "pending",
			CreateTime:    now.Add(-4 * time.Second).UnixNano(),
			ModifyTime:    now.UnixNano(),
		},
	}

	cmd.formatEvalStatus(eval, placed, false, shortId)
	out := ui.OutputWriter.String()

	// there isn't much logic here, so this is just a smoke test
	must.StrContains(t, out, `
Failed Placements
Task Group "web" (failed to place 1 allocation):
  * Constraint "${attr.kernel.name} = linux": 2 nodes excluded by filter
  * Resources exhausted on 2 nodes
  * Dimension "memory" exhausted on 2 nodes`)

	must.StrContains(t, out, `Related Evaluations`)
	must.StrContains(t, out, `Placed Allocations`)
}
