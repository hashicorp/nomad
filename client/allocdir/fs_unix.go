// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package allocdir

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

var (
	// SharedAllocContainerPath is the path inside container for mounted
	// directory shared across tasks in a task group.
	SharedAllocContainerPath = filepath.Join("/", SharedAllocName)

	// TaskLocalContainer is the path inside a container for mounted directory
	// for local storage.
	TaskLocalContainerPath = filepath.Join("/", TaskLocal)

	// TaskSecretsContainerPath is the path inside a container for mounted
	// secrets directory
	TaskSecretsContainerPath = filepath.Join("/", TaskSecrets)
)

// dropDirPermissions gives full access to a directory to all users and sets
// the owner to nobody.
func dropDirPermissions(path string, desired os.FileMode) error {
	if err := os.Chmod(path, desired|0777); err != nil {
		return fmt.Errorf("Chmod(%v) failed: %v", path, err)
	}

	// Can't change owner if not root.
	if unix.Geteuid() != 0 {
		return nil
	}

	u, err := user.Lookup("nobody")
	if err != nil {
		return err
	}

	uid, err := getUid(u)
	if err != nil {
		return err
	}

	gid, err := getGid(u)
	if err != nil {
		return err
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("Couldn't change owner/group of %v to (uid: %v, gid: %v): %v", path, uid, gid, err)
	}

	return nil
}

// getUid for a user
func getUid(u *user.User) (int, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Uid to an int: %v", err)
	}

	return uid, nil
}

// getGid for a user
func getGid(u *user.User) (int, error) {
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Gid to an int: %v", err)
	}

	return gid, nil
}

// linkOrCopy attempts to hardlink dst to src and fallsback to copying if the
// hardlink fails.
func linkOrCopy(src, dst string, uid, gid int, perm os.FileMode) error {
	// Avoid link/copy if the file already exists in the chroot
	// TODO 0.6 clean this up. This was needed because chroot creation fails
	// when a process restarts.
	if fileInfo, _ := os.Stat(dst); fileInfo != nil {
		return nil
	}
	// Attempt to hardlink.
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	return fileCopy(src, dst, uid, gid, perm)
}

func getOwner(fi os.FileInfo) (int, int) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, -1
	}
	return int(stat.Uid), int(stat.Gid)
}
