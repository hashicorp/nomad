// +build !windows

package allocdir

import (
	"fmt"
	"github.com/hashicorp/nomad/nomad/structs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

func (d *AllocDir) Build(tasks []*structs.Task) error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.Mkdir(d.AllocDir, 0700); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %v", d.AllocDir, err)
	}

	nobody, err := user.Lookup("nobody")
	if err != nil {
		return fmt.Errorf("Could not set owner/group on shared alloc directory: %v", err)
	}

	uid, err := getUid(nobody)
	if err != nil {
		return err
	}

	gid, err := getGid(nobody)
	if err != nil {
		return err
	}

	// Make the shared directory and make it availabe to all user/groups.
	if err := mkOwnedDir(d.SharedDir, uid, gid, 0777); err != nil {
		return err
	}

	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := mkOwnedDir(p, uid, gid, 0777); err != nil {
			return err
		}
	}

	// Make the task directories.
	for _, t := range tasks {
		p := filepath.Join(d.AllocDir, t.Name)
		if err := mkOwnedDir(p, uid, gid, 0777); err != nil {
			return err
		}

		// Create a local directory that each task can use.
		local := filepath.Join(p, TaskLocal)
		if err := mkOwnedDir(local, uid, gid, 0777); err != nil {
			return err
		}
		d.TaskDirs[t.Name] = local

		// TODO: Mount the shared alloc dir into each task dir.
	}

	return nil
}

// mkOwnedDir creates the directory specified by the path with the passed
// permissions. It also sets the passed uid and gid. It returns an error if any
// of these operations fail.
func mkOwnedDir(path string, uid, gid int, perm os.FileMode) error {
	if err := os.Mkdir(path, perm); err != nil {
		return fmt.Errorf("Failed to make directory %v: %v", path, err)
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("Couldn't change owner/group of %v to (uid: %v, gid: %v): %v", path, uid, gid, err)
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
