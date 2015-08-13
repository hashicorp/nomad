package scheduler

// Context is used to track contextual information used for placement
type Context interface {
	// State is used to inspect the current global state
	State() State
}

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
	state State
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext(s State) *EvalContext {
	ctx := &EvalContext{}
	return ctx
}

func (e *EvalContext) State() State {
	return e.state
}

func (e *EvalContext) SetState(s State) {
	e.state = s
}
