// +build !windows

// Functions shared between linux/darwin.
package allocdir

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func (d *AllocDir) linkOrCopy(src, dst string, perm os.FileMode) error {
	// Attempt to hardlink.
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	return fileCopy(src, dst, perm)
}

func (d *AllocDir) dropDirPermissions(path string) error {
	// Can't do anything if not root.
	if syscall.Geteuid() != 0 {
		return nil
	}

	u, err := userLookup("nobody")
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

	if err := os.Chmod(path, 0777); err != nil {
		return fmt.Errorf("Chmod(%v) failed: %v", path, err)
	}

	return nil
}

func getUid(u *user.User) (int, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Uid to an int: %v", err)
	}

	return uid, nil
}

func getGid(u *user.User) (int, error) {
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Gid to an int: %v", err)
	}

	return gid, nil
}

// userLookup checks if the given username or uid is present in /etc/passwd
// and returns the user struct.
// If the username is not found, an error is returned.
// Credit to @creak, https://github.com/docker/docker/pull/1096
func userLookup(uid string) (*user.User, error) {
	file, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(file), "\n") {
		data := strings.Split(line, ":")
		if len(data) > 5 && (data[0] == uid || data[2] == uid) {
			return &user.User{
				Uid:      data[2],
				Gid:      data[3],
				Username: data[0],
				Name:     data[4],
				HomeDir:  data[5],
			}, nil
		}
	}
	return nil, fmt.Errorf("User not found in /etc/passwd")
}
