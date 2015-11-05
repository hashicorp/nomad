// +build !linux

package executor

func NewExecutor() Executor {
	return &UniversalExecutor{BasicExecutor{}}
}

// UniversalExecutor wraps the BasicExecutor
type UniversalExecutor struct {
	BasicExecutor
}
