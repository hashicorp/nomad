package nomad

import (
	"github.com/hashicorp/go-hclog"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

type PlanEnqueuer interface {
	Enqueue(plan *structs.Plan) (PlanFuture, error)
}

type plannerLocal struct {
	enq    PlanEnqueuer
	logger hclog.Logger

	Result   *structs.PlanResult
	Eval     *structs.Evaluation
	NextEval *structs.Evaluation
}

func NewLocalPlanner(logger hclog.Logger, enq PlanEnqueuer) *plannerLocal {
	return &plannerLocal{
		enq:    enq,
		logger: logger,
	}
}

// SubmitPlan is used to submit a plan for consideration.
// This will return a PlanResult or an error. It is possible
// that this will result in a state refresh as well.
func (p *plannerLocal) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, scheduler.State, error) {
	// Submit the plan to the queue
	future, err := p.enq.Enqueue(plan)
	if err != nil {
		return nil, nil, err
	}

	// Wait for the results
	result, err := future.Wait()
	if err != nil {
		return nil, nil, err
	}

	p.Result = result
	return result, nil, nil
}

// UpdateEval is used to update an evaluation. This should update
// a copy of the input evaluation since that should be immutable.
func (p *plannerLocal) UpdateEval(eval *structs.Evaluation) error {
	p.Eval = eval
	return nil
}

// CreateEval is used to create an evaluation. This should set the
// PreviousEval to that of the current evaluation.
func (p *plannerLocal) CreateEval(eval *structs.Evaluation) error {
	p.NextEval = eval
	return nil
}

// ReblockEval takes a blocked evaluation and re-inserts it into the blocked
// evaluation tracker. This update occurs only in-memory on the leader. The
// evaluation must exist in a blocked state prior to this being called such
// that on leader changes, the evaluation will be reblocked properly.
func (p *plannerLocal) ReblockEval(_ *structs.Evaluation) error {
	panic("not implemented") // TODO: Implement
	return nil
}

// ServersMeetMinimumVersion is always true locally
func (p *plannerLocal) ServersMeetMinimumVersion(minVersion *version.Version, checkFailedServers bool) bool {
	return true
}
