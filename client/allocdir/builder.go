package allocdir

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (d *AllocDir) Build(tasks []*structs.Task) error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.MkdirAll(d.AllocDir, 0700); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %v", d.AllocDir, err)
	}

	// Check if the process is has root capabilities and if so set the
	// user/group to nobody.
	var u *user.User
	if syscall.Geteuid() == 0 {
		nobody, err := user.Lookup("nobody")
		if err != nil {
			return fmt.Errorf("Could not set owner/group on shared alloc directory: %v", err)
		}
		u = nobody
	}

	// Make the shared directory and make it availabe to all user/groups.
	if err := mkOwnedDir(d.SharedDir, u, 0777); err != nil {
		return err
	}

	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := mkOwnedDir(p, u, 0777); err != nil {
			return err
		}
	}

	// Make the task directories.
	for _, t := range tasks {
		p := filepath.Join(d.AllocDir, t.Name)
		if err := mkOwnedDir(p, u, 0777); err != nil {
			return err
		}

		// Create a local directory that each task can use.
		local := filepath.Join(p, TaskLocal)
		if err := mkOwnedDir(local, u, 0777); err != nil {
			return err
		}
		d.TaskDirs[t.Name] = p

		// TODO: Mount the shared alloc dir into each task dir.
	}

	return nil
}

func (d *AllocDir) Embed(task string, dirs map[string]string) error {
	taskdir, ok := d.TaskDirs[task]
	if !ok {
		return fmt.Errorf("Task directory doesn't exist for task %v", task)
	}

	subdirs := make(map[string]string)
	for source, dest := range dirs {
		// Enumerate the files in source.
		entries, err := ioutil.ReadDir(source)
		if err != nil {
			return fmt.Errorf("Couldn't read directory %v: %v", source, err)
		}

		// Create destination directory.
		destDir := filepath.Join(taskdir, dest)
		if err := os.MkdirAll(destDir, 0777); err != nil {
			return fmt.Errorf("Couldn't create destination directory %v: %v", destDir, err)
		}

		for _, entry := range entries {
			hostEntry := filepath.Join(source, entry.Name())
			if entry.IsDir() {
				subdirs[hostEntry] = filepath.Join(dest, filepath.Base(hostEntry))
				continue
			} else if !entry.Mode().IsRegular() {
				return fmt.Errorf("Can't embed non-regular file: %v", hostEntry)
			}

			taskEntry := filepath.Join(destDir, filepath.Base(hostEntry))

			// Attempt to hardlink.
			if err := os.Link(hostEntry, taskEntry); err == nil {
				continue
			}

			// Do a simple copy.
			src, err := os.Open(hostEntry)
			if err != nil {
				return fmt.Errorf("Couldn't open host file %v: %v", hostEntry, err)
			}

			dst, err := os.OpenFile(taskEntry, os.O_WRONLY|os.O_CREATE, 0777)
			if err != nil {
				return fmt.Errorf("Couldn't create task file %v: %v", taskEntry, err)
			}

			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("Couldn't copy %v to %v: %v", hostEntry, taskEntry, err)
			}
		}
	}

	// Recurse on self to copy subdirectories.
	if len(subdirs) != 0 {
		return d.Embed(task, subdirs)
	}

	return nil
}

// mkOwnedDir creates the directory specified by the path with the passed
// permissions. It also sets the user/group based on the passed user if it is
// non-nil. It returns an error if any of these operations fail.
func mkOwnedDir(path string, user *user.User, perm os.FileMode) error {
	if err := os.Mkdir(path, perm); err != nil {
		return fmt.Errorf("Failed to make directory %v: %v", path, err)
	}

	if user == nil {
		return nil
	}

	uid, err := getUid(user)
	if err != nil {
		return err
	}

	gid, err := getGid(user)
	if err != nil {
		return err
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
