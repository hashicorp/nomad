package allocdir

import (
	"syscall"
)

// Hardlinks the shared directory. As a side-effect the src and dest directory
// must be on the same filesystem.
func (d *AllocDir) mount(src, dest string) error {
	return syscall.Link(src, dest)
}

func (d *AllocDir) unmount(dir string) error {
	return syscall.Unlink(dir)
}

// MountSpecialDirs mounts the dev and proc file system on the chroot of the
// task. It's a no-op on darwin.
func (d *AllocDir) MountSpecialDirs(taskDir string) error {
	return nil
}

// unmountSpecialDirs unmounts the dev and proc file system from the chroot
func (d *AllocDir) unmountSpecialDirs(taskDir string) error {
	return nil
}
