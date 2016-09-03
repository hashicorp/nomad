package allocdir

import (
	"os"
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

// createSecretDir creates the secrets dir folder at the given path
func (d *AllocDir) createSecretDir(dir string) error {
	return os.MkdirAll(dir, 0777)
}

// removeSecretDir removes the secrets dir folder
func (d *AllocDir) removeSecretDir(dir string) error {
	return os.RemoveAll(dir)
}

// MountSpecialDirs mounts the dev and proc file system on the chroot of the
// task. It's a no-op on FreeBSD right now.
func (d *AllocDir) MountSpecialDirs(taskDir string) error {
	return nil
}

// unmountSpecialDirs unmounts the dev and proc file system from the chroot
func (d *AllocDir) unmountSpecialDirs(taskDir string) error {
	return nil
}
