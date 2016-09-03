package allocdir

import (
	"errors"
	"os"
	"path/filepath"
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

// The windows version does nothing currently.
func (d *AllocDir) mountSharedDir(dir string) error {
	return errors.New("Mount on Windows not supported.")
}

// createSecretDir creates the secrets dir folder at the given path
func (d *AllocDir) createSecretDir(dir string) error {
	return os.MkdirAll(dir, 0777)
}

// removeSecretDir removes the secrets dir folder
func (d *AllocDir) removeSecretDir(dir string) error {
	return os.RemoveAll(dir)
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

// unmountSpecialDirs unmounts the dev and proc file system from the chroot
func (d *AllocDir) unmountSpecialDirs(taskDir string) error {
	return nil
}
