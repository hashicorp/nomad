package executor

import (
	"os"
	"os/exec"
)

// TODO Figure out if this is needed in Windows
func isolateCommand(cmd *exec.Cmd) {}

func isProcessRunning(process *os.Process) bool {
	return true
}
