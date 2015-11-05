package executor

import "testing"

func TestExecutorBasic(t *testing.T) {
	testExecutor(t, NewBasicExecutor, nil)
}
