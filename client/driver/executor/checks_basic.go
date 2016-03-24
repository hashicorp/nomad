// +build !linux

package executor

import (
	"os/exec"
)

func (e *ExecScriptCheck) setChroot(cmd *exec.Cmd) {
}
