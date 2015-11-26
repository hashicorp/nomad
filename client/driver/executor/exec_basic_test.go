package executor

import "testing"

func TestExecutorBasic(t *testing.T) {
	t.Parallel()
	testExecutor(t, NewBasicExecutor, nil)
}
