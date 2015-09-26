package allocdir

import (
	"syscall"
)

// Hardlinks the shared directory. As a side-effect the shared directory and
// task directory must be on the same filesystem.
func (d *AllocDir) mountSharedDir(dir string) error {
	return syscall.Link(d.SharedDir, dir)
}

func (d *AllocDir) unmountSharedDir(dir string) error {
	return syscall.Unlink(dir)
}
