package executor

import (
	"testing"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func TestExecutorLinux(t *testing.T) {
	testExecutor(t, NewLinuxExecutor, ctestutil.ExecCompatible)
}
