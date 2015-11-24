package executor

import (
	"testing"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testtask"
)

func init() {
	// Add test binary to chroot during test run.
	chrootEnv[testtask.Path()] = testtask.Path()
}

func TestExecutorLinux(t *testing.T) {
	t.Parallel()
	testExecutor(t, NewLinuxExecutor, ctestutil.ExecCompatible)
}
