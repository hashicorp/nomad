package scheduler

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext() *EvalContext {
	ctx := &EvalContext{}
	return ctx
}
