package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
	"pgregory.net/rapid"
)

func TestScheduler_Prop(t *testing.T) {

	rapid.Check(t, func(rt *rapid.T) {
		gen := genHarnessFactory(t, "service")
		in := gen.Draw(rt, "in") // TODO: what's the label here?
		err := in.Harness.Process(NewServiceScheduler, in.Eval)
		must.NoError(rt, err)
		must.Len(rt, 1, in.Harness.Plans)
	})
}

type propTestInput struct {
	Harness *tests.Harness
	Eval    *structs.Evaluation
}

func genHarnessFactory(tt *testing.T, schedType string) *rapid.Generator[*propTestInput] {
	// TODO: how do we get *rapid.T to work with testing.TB?
	return rapid.Custom(func(t *rapid.T) *propTestInput {

		h := tests.NewHarness(tt)
		eval := rapid.Make[*structs.Evaluation]().Draw(t, "eval")
		// job := rapid.Make[*structs.Job]().Draw(t, "job")

		// eval.JobID = job.ID
		// eval.Namespace = job.Namespace

		// must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

		return &propTestInput{
			Harness: h,
			Eval:    eval,
		}
	})
}
