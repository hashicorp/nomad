package executor

import (
	"os"
	"os/exec"
)

// TODO Figure out if this is needed in Windows
func isolateCommand(cmd *exec.Cmd) {}

// findLiveProcess looks up a given pid and attempts to validate its liveness.
func findLiveProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}
