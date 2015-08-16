package scheduler

import (
	"log"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Context is used to track contextual information used for placement
type Context interface {
	// State is used to inspect the current global state
	State() State

	// Plan returns the current plan
	Plan() *structs.Plan

	// Logger provides a way to log
	Logger() *log.Logger

	// Metrics returns the current metrics
	Metrics() *structs.AllocMetric

	// Reset is invoked after making a placement
	Reset()

	// ProposedAllocs returns the proposed allocations for a node
	// which is the existing allocations, removing evictions, and
	// adding any planned placements.
	ProposedAllocs(nodeID string) ([]*structs.Allocation, error)
}

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
	state   State
	plan    *structs.Plan
	logger  *log.Logger
	metrics *structs.AllocMetric
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext(s State, p *structs.Plan, log *log.Logger) *EvalContext {
	ctx := &EvalContext{
		state:   s,
		plan:    p,
		logger:  log,
		metrics: new(structs.AllocMetric),
	}
	return ctx
}

func (e *EvalContext) State() State {
	return e.state
}

func (e *EvalContext) Plan() *structs.Plan {
	return e.plan
}

func (e *EvalContext) Logger() *log.Logger {
	return e.logger
}

func (e *EvalContext) Metrics() *structs.AllocMetric {
	return e.metrics
}

func (e *EvalContext) SetState(s State) {
	e.state = s
}

func (e *EvalContext) Reset() {
	e.metrics = new(structs.AllocMetric)
}

func (e *EvalContext) ProposedAllocs(nodeID string) ([]*structs.Allocation, error) {
	// Get the existing allocations
	existingAlloc, err := e.state.AllocsByNode(nodeID)
	if err != nil {
		return nil, err
	}

	// Determine the proposed allocation by first removing allocations
	// that are planned evictions and adding the new allocations.
	proposed := existingAlloc
	if evict := e.plan.NodeEvict[nodeID]; len(evict) > 0 {
		proposed = structs.RemoveAllocs(existingAlloc, evict)
	}
	proposed = append(proposed, e.plan.NodeAllocation[nodeID]...)

	// Ensure the return is not nil
	if proposed == nil {
		proposed = make([]*structs.Allocation, 0)
	}
	return proposed, nil
}
