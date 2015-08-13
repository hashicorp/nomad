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
}

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
	state  State
	plan   *structs.Plan
	logger *log.Logger
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext(s State, p *structs.Plan, log *log.Logger) *EvalContext {
	ctx := &EvalContext{
		state:  s,
		plan:   p,
		logger: log,
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

func (e *EvalContext) SetState(s State) {
	e.state = s
}
