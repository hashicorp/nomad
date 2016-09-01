package allocdir

import (
	"os"
	"path/filepath"
	"syscall"
)

var (
	// SharedAllocContainerPath is the path inside container for mounted
	// directory shared across tasks in a task group.
	SharedAllocContainerPath = filepath.Join("c:\\", SharedAllocName)

	// TaskLocalContainer is the path inside a container for mounted directory
	// for local storage.
	TaskLocalContainerPath = filepath.Join("c:\\", TaskLocal)

	// TaskSecretsContainerPath is the path inside a container for mounted
	// secrets directory
	TaskSecretsContainerPath = filepath.Join("c:\\", TaskSecrets)
)

func (d *AllocDir) linkOrCopy(src, dst string, perm os.FileMode) error {
	return fileCopy(src, dst, perm)
}

// Hardlinks the shared directory. As a side-effect the src and dest directory
// must be on the same filesystem.
func (d *AllocDir) mount(src, dest string) error {
	return os.Symlink(src, dest)
}

func (d *AllocDir) unmount(dir string) error {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	return syscall.RemoveDirectory(p)
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
