package diff

import "github.com/hashicorp/nomad/nomad/structs"

// JobPlanResponse is used to respond to a job plan request. Must be in this
// package to avoid a cyclic dependency.
type JobPlanResponse struct {
	// Plan holds the decisions the scheduler made.
	Plan *structs.Plan

	// The Cas value can be used when running `nomad run` to ensure that the Job
	// wasn’t modified since the last plan. If the job is being created, the
	// value is zero.
	Cas uint64

	// CreatedEvals is the set of evaluations created by the scheduler. The
	// reasons for this can be rolling-updates or blocked evals.
	CreatedEvals []*structs.Evaluation

	// Diff contains the diff of the job and annotations on whether the change
	// causes an in-place update or create/destroy
	Diff *JobDiff

	structs.QueryMeta
}
