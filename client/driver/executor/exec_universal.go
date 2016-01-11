// +build !linux

package executor

func NewExecutor(ctx *ExecutorContext) Executor {
	return &UniversalExecutor{
		BasicExecutor: NewBasicExecutor(ctx).(*BasicExecutor),
	}
}

// UniversalExecutor wraps the BasicExecutor
type UniversalExecutor struct {
	*BasicExecutor
}
