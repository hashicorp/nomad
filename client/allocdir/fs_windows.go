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

// linkOrCopy is always copies dst to src on Windows.
func linkOrCopy(src, dst string, uid, gid int, perm os.FileMode) error {
	return fileCopy(src, dst, uid, gid, perm)
}

// The windows version does nothing currently.
func mountSharedDir(dir string) error {
	return errors.New("Mount on Windows not supported.")
}

// The windows version does nothing currently.
func linkDir(src, dst string) error {
	return nil
}

// The windows version does nothing currently.
func unlinkDir(dir string) error {
	return nil
}

// createSecretDir creates the secrets dir folder at the given path
func createSecretDir(dir string) error {
	return os.MkdirAll(dir, 0777)
}

// removeSecretDir removes the secrets dir folder
func removeSecretDir(dir string) error {
	return os.RemoveAll(dir)
}

// The windows version does nothing currently.
func dropDirPermissions(path string, desired os.FileMode) error {
	return nil
}

// The windows version does nothing currently.
func unmountSharedDir(dir string) error {
	return nil
}

// MountSpecialDirs mounts the dev and proc file system on the chroot of the
// task. It's a no-op on windows.
func MountSpecialDirs(taskDir string) error {
	return nil
}

// unmountSpecialDirs unmounts the dev and proc file system from the chroot
func unmountSpecialDirs(taskDir string) error {
	return nil
}

// getOwner doesn't work on Windows as Windows doesn't use int user IDs
func getOwner(os.FileInfo) (int, int) {
	return idUnsupported, idUnsupported
}
