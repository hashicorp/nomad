package scheduler

// Context is used to track contextual information used for placement
type Context interface {
}

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext() *EvalContext {
	ctx := &EvalContext{}
	return ctx
}
