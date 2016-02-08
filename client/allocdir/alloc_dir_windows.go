package allocdir

import (
	"errors"
	"os"
)

func (d *AllocDir) linkOrCopy(src, dst string, perm os.FileMode) error {
	return fileCopy(src, dst, perm)
}

// The windows version does nothing currently.
func (d *AllocDir) mountSharedDir(dir string) error {
	return errors.New("Mount on Windows not supported.")
}

// The windows version does nothing currently.
func (d *AllocDir) dropDirPermissions(path string) error {
	return nil
}

// The windows version does nothing currently.
func (d *AllocDir) unmountSharedDir(dir string) error {
	return nil
}

// MountSpecialDirs mounts the dev and proc file system on the chroot of the
// task. It's a no-op on windows.
func (d *AllocDir) MountSpecialDirs(taskDir string) error {
	return nil
}

// UnmountSpecialDirs unmounts the dev and proc file system from the chroot
func (d *AllocDir) UnmountSpecialDirs(taskDir string) error {
	return nil
}
