package allocdir

import (
	//"os"
	"path/filepath"
	"syscall"
)

// Hardlinks the shared directory. As a side-effect the shared directory and
// task directory must be on the same filesystem.
func (d *AllocDir) mountSharedDir(taskDir string) error {
	taskLoc := filepath.Join(taskDir, SharedAllocName)
	return syscall.Link(d.SharedDir, taskLoc)
}
