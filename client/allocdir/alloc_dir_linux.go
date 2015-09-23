package allocdir

import (
	"os"
	"path/filepath"
	"syscall"
)

// Bind mounts the shared directory into the task directory. Must be root to
// run.
func (d *AllocDir) mountSharedDir(taskDir string) error {
	taskLoc := filepath.Join(taskDir, SharedAllocName)
	if err := os.Mkdir(taskLoc, 0777); err != nil {
		return err
	}

	return syscall.Mount(d.SharedDir, taskLoc, "", syscall.MS_BIND, "")
}
