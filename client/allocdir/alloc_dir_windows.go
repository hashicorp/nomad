package allocdir

import (
	"os"
	"path/filepath"
	"syscall"
)

var (
	//Path inside container for mounted directory that is shared across tasks in a task group.
	SharedAllocContainerPath = filepath.Join("c:\\", SharedAllocName)

	//Path inside container for mounted directory for local storage.
	TaskLocalContainerPath = filepath.Join("c:\\", TaskLocal)
)

func (d *AllocDir) linkOrCopy(src, dst string, perm os.FileMode) error {
	return fileCopy(src, dst, perm)
}

// Hardlinks the shared directory. As a side-effect the src and dest directory
// must be on the same filesystem.
func (d *AllocDir) mount(src, dest string) error {
	return syscall.Link(src, dest)
}

func (d *AllocDir) unmount(dir string) error {
	return syscall.Unlink(dir)
}

// The windows version does nothing currently.
func (d *AllocDir) dropDirPermissions(path string) error {
	return nil
}

// MountSpecialDirs mounts the dev and proc file system on the chroot of the
// task. It's a no-op on windows.
func (d *AllocDir) MountSpecialDirs(taskDir string) error {
	return nil
}

// unmountSpecialDirs unmounts the dev and proc file system from the chroot
func (d *AllocDir) unmountSpecialDirs(taskDir string) error {
	return nil
}
