package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

// InMemPlanner is an in-memory Planner that can be used to invoke the scheduler
// without side-effects.
type InMemPlanner struct {
	CreatedEvals []*structs.Evaluation
	Plan         *structs.Plan
}

// NewInMemoryPlanner returns a new in-memory planner.
func NewInMemoryPlanner() *InMemPlanner {
	return &InMemPlanner{}
}

func (i *InMemPlanner) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, State, error) {
	i.Plan = plan

	// Create a fully committed plan result.
	result := &structs.PlanResult{
		NodeUpdate:     plan.NodeUpdate,
		NodeAllocation: plan.NodeAllocation,
	}

	return result, nil, nil
}

func (i *InMemPlanner) UpdateEval(eval *structs.Evaluation) error {
	return nil
}

func (i *InMemPlanner) CreateEval(eval *structs.Evaluation) error {
	i.CreatedEvals = append(i.CreatedEvals, eval)
	return nil
}
