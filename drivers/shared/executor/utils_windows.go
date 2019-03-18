package executor

import (
	"os"
	"os/exec"
)

// TODO Figure out if this is needed in Windows
func isolateCommand(cmd *exec.Cmd) {}

func isProcessRunning(pid int) bool {
	// on Windows, FindProcess returns an error if proc is not found
	_, err := os.FindProcess(pid)
	return err == nil
}
