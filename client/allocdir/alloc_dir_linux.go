package allocdir

import (
	"os"
	"syscall"
)

// Bind mounts the shared directory into the task directory. Must be root to
// run.
func (d *AllocDir) mountSharedDir(taskDir string) error {
	if err := os.MkdirAll(taskDir, 0777); err != nil {
		return err
	}

	return syscall.Mount(d.SharedDir, taskDir, "", syscall.MS_BIND, "")
}

func (d *AllocDir) unmountSharedDir(dir string) error {
	return syscall.Unmount(dir, 0)
}
