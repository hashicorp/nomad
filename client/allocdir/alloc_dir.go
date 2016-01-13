package allocdir

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// The set of directories that exist inside eache shared alloc directory.
	SharedAllocDirs = []string{"logs", "tmp", "data"}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"
)

type AllocDir struct {
	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string

	// The shared directory is available to all tasks within the same task
	// group.
	SharedDir string

	// TaskDirs is a mapping of task names to their non-shared directory.
	TaskDirs map[string]string

	// A list of locations the shared alloc has been mounted to.
	mounted []string
}

type AllocFile struct {
	Name  string
	IsDir bool
	Size  int64
}

func NewAllocDir(allocDir string) *AllocDir {
	d := &AllocDir{AllocDir: allocDir, TaskDirs: make(map[string]string)}
	d.SharedDir = filepath.Join(d.AllocDir, SharedAllocName)
	return d
}

// Tears down previously build directory structure.
func (d *AllocDir) Destroy() error {
	// Unmount all mounted shared alloc dirs.
	for _, m := range d.mounted {
		if err := d.unmountSharedDir(m); err != nil {
			return fmt.Errorf("Failed to unmount shared directory: %v", err)
		}
	}

	return os.RemoveAll(d.AllocDir)
}

// Given a list of a task build the correct alloc structure.
func (d *AllocDir) Build(tasks []*structs.Task) error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.MkdirAll(d.AllocDir, 0700); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %v", d.AllocDir, err)
	}

	// Make the shared directory and make it availabe to all user/groups.
	if err := os.Mkdir(d.SharedDir, 0777); err != nil {
		return err
	}

	// Make the shared directory have non-root permissions.
	if err := d.dropDirPermissions(d.SharedDir); err != nil {
		return err
	}

	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := os.Mkdir(p, 0777); err != nil {
			return err
		}
	}

	// Make the task directories.
	for _, t := range tasks {
		taskDir := filepath.Join(d.AllocDir, t.Name)
		if err := os.Mkdir(taskDir, 0777); err != nil {
			return err
		}

		// Make the task directory have non-root permissions.
		if err := d.dropDirPermissions(taskDir); err != nil {
			return err
		}

		// Create a local directory that each task can use.
		local := filepath.Join(taskDir, TaskLocal)
		if err := os.Mkdir(local, 0777); err != nil {
			return err
		}

		if err := d.dropDirPermissions(local); err != nil {
			return err
		}

		d.TaskDirs[t.Name] = taskDir
	}

	return nil
}

// Embed takes a mapping of absolute directory or file paths on the host to
// their intended, relative location within the task directory. Embed attempts
// hardlink and then defaults to copying. If the path exists on the host and
// can't be embeded an error is returned.
func (d *AllocDir) Embed(task string, entries map[string]string) error {
	taskdir, ok := d.TaskDirs[task]
	if !ok {
		return fmt.Errorf("Task directory doesn't exist for task %v", task)
	}

	subdirs := make(map[string]string)
	for source, dest := range entries {
		// Check to see if directory exists on host.
		s, err := os.Stat(source)
		if os.IsNotExist(err) {
			continue
		}

		// Embedding a single file
		if !s.IsDir() {
			destDir := filepath.Join(taskdir, filepath.Dir(dest))
			if err := os.MkdirAll(destDir, s.Mode().Perm()); err != nil {
				return fmt.Errorf("Couldn't create destination directory %v: %v", destDir, err)
			}

			// Copy the file.
			taskEntry := filepath.Join(destDir, filepath.Base(dest))
			if err := d.linkOrCopy(source, taskEntry, s.Mode().Perm()); err != nil {
				return err
			}

			continue
		}

		// Create destination directory.
		destDir := filepath.Join(taskdir, dest)
		if err := os.MkdirAll(destDir, s.Mode().Perm()); err != nil {
			return fmt.Errorf("Couldn't create destination directory %v: %v", destDir, err)
		}

		// Enumerate the files in source.
		dirEntries, err := ioutil.ReadDir(source)
		if err != nil {
			return fmt.Errorf("Couldn't read directory %v: %v", source, err)
		}

		for _, entry := range dirEntries {
			hostEntry := filepath.Join(source, entry.Name())
			taskEntry := filepath.Join(destDir, filepath.Base(hostEntry))
			if entry.IsDir() {
				subdirs[hostEntry] = filepath.Join(dest, filepath.Base(hostEntry))
				continue
			}

			// Check if entry exists. This can happen if restarting a failed
			// task.
			if _, err := os.Lstat(taskEntry); err == nil {
				continue
			}

			if !entry.Mode().IsRegular() {
				// If it is a symlink we can create it, otherwise we skip it.
				if entry.Mode()&os.ModeSymlink == 0 {
					continue
				}

				link, err := os.Readlink(hostEntry)
				if err != nil {
					return fmt.Errorf("Couldn't resolve symlink for %v: %v", source, err)
				}

				if err := os.Symlink(link, taskEntry); err != nil {
					return fmt.Errorf("Couldn't create symlink: %v", err)
				}
				continue
			}

			if err := d.linkOrCopy(hostEntry, taskEntry, entry.Mode().Perm()); err != nil {
				return err
			}
		}
	}

	// Recurse on self to copy subdirectories.
	if len(subdirs) != 0 {
		return d.Embed(task, subdirs)
	}

	return nil
}

// MountSharedDir mounts the shared directory into the specified task's
// directory. Mount is documented at an OS level in their respective
// implementation files.
func (d *AllocDir) MountSharedDir(task string) error {
	taskDir, ok := d.TaskDirs[task]
	if !ok {
		return fmt.Errorf("No task directory exists for %v", task)
	}

	taskLoc := filepath.Join(taskDir, SharedAllocName)
	if err := d.mountSharedDir(taskLoc); err != nil {
		return fmt.Errorf("Failed to mount shared directory for task %v: %v", task, err)
	}

	d.mounted = append(d.mounted, taskLoc)
	return nil
}

func (d *AllocDir) FSList(path string) ([]*AllocFile, error) {
	p := filepath.Join(d.AllocDir, path)
	finfos, err := ioutil.ReadDir(p)
	if err != nil {
		return []*AllocFile{}, nil
	}
	files := make([]*AllocFile, len(finfos))
	for idx, info := range finfos {
		files[idx] = &AllocFile{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}
	}
	return files, err
}

func (d *AllocDir) FSStat(path string) (*AllocFile, error) {
	p := filepath.Join(d.AllocDir, path)
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	return &AllocFile{
		Size:  info.Size(),
		Name:  info.Name(),
		IsDir: info.IsDir(),
	}, nil
}

func (d *AllocDir) FSReadAt(allocID string, path string, offset int64, limit int64, w io.Writer) error {
	p := filepath.Join(d.AllocDir, path)
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	io.Copy(w, io.LimitReader(f, limit))
	return nil
}

func fileCopy(src, dst string, perm os.FileMode) error {
	// Do a simple copy.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Couldn't open src file %v: %v", src, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("Couldn't create destination file %v: %v", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Couldn't copy %v to %v: %v", src, dst, err)
	}

	return nil
}
