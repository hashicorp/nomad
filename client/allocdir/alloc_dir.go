package allocdir

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/tomb.v1"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
)

var (
	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// Name of the directory where logs of Tasks are written
	LogDirName = "logs"

	// SharedDataDir is one of the shared allocation directories. It is
	// included in snapshots.
	SharedDataDir = "data"

	// The set of directories that exist inside eache shared alloc directory.
	SharedAllocDirs = []string{LogDirName, "tmp", SharedDataDir}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"

	// TaskSecrets is the name of the secret directory inside each task
	// directory
	TaskSecrets = "secrets"

	// TaskDirs is the set of directories created in each tasks directory.
	TaskDirs = []string{"tmp"}
)

type AllocDir struct {
	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string

	// The shared directory is available to all tasks within the same task
	// group.
	SharedDir string

	// TaskDirs is a mapping of task names to their non-shared directory.
	TaskDirs map[string]*TaskDir

	logger *log.Logger
}

// AllocFileInfo holds information about a file inside the AllocDir
type AllocFileInfo struct {
	Name     string
	IsDir    bool
	Size     int64
	FileMode string
	ModTime  time.Time
}

// AllocDirFS exposes file operations on the alloc dir
type AllocDirFS interface {
	List(path string) ([]*AllocFileInfo, error)
	Stat(path string) (*AllocFileInfo, error)
	ReadAt(path string, offset int64) (io.ReadCloser, error)
	Snapshot(w io.Writer) error
	BlockUntilExists(path string, t *tomb.Tomb) (chan error, error)
	ChangeEvents(path string, curOffset int64, t *tomb.Tomb) (*watch.FileChanges, error)
}

// NewAllocDir initializes the AllocDir struct with allocDir as base path for
// the allocation directory.
func NewAllocDir(logger *log.Logger, allocDir string) *AllocDir {
	return &AllocDir{
		AllocDir:  allocDir,
		SharedDir: filepath.Join(allocDir, SharedAllocName),
		TaskDirs:  make(map[string]*TaskDir),
		logger:    logger,
	}
}

// NewTaskDir creates a new TaskDir and adds it to the AllocDirs TaskDirs map.
func (d *AllocDir) NewTaskDir(name string) *TaskDir {
	td := newTaskDir(d.logger, d.AllocDir, name)
	d.TaskDirs[name] = td
	return td
}

// Snapshot creates an archive of the files and directories in the data dir of
// the allocation and the task local directories
func (d *AllocDir) Snapshot(w io.Writer) error {
	allocDataDir := filepath.Join(d.SharedDir, SharedDataDir)
	rootPaths := []string{allocDataDir}
	for _, taskdir := range d.TaskDirs {
		rootPaths = append(rootPaths, taskdir.LocalDir)
	}

	tw := tar.NewWriter(w)
	defer tw.Close()

	walkFn := func(path string, fileInfo os.FileInfo, err error) error {
		// Ignore if the file is a symlink
		if fileInfo.Mode() == os.ModeSymlink {
			return nil
		}

		// Include the path of the file name relative to the alloc dir
		// so that we can put the files in the right directories
		relPath, err := filepath.Rel(d.AllocDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			return fmt.Errorf("error creating file header: %v", err)
		}
		hdr.Name = relPath
		tw.WriteHeader(hdr)

		// If it's a directory we just write the header into the tar
		if fileInfo.IsDir() {
			return nil
		}

		// Write the file into the archive
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	}

	// Walk through all the top level directories and add the files and
	// directories in the archive
	for _, path := range rootPaths {
		if err := filepath.Walk(path, walkFn); err != nil {
			return err
		}
	}

	return nil
}

// Move other alloc directory's shared path and local dir to this alloc dir.
func (d *AllocDir) Move(other *AllocDir, tasks []*structs.Task) error {
	// Move the data directory
	otherDataDir := filepath.Join(other.SharedDir, SharedDataDir)
	dataDir := filepath.Join(d.SharedDir, SharedDataDir)
	if fileInfo, err := os.Stat(otherDataDir); fileInfo != nil && err == nil {
		if err := os.Rename(otherDataDir, dataDir); err != nil {
			return fmt.Errorf("error moving data dir: %v", err)
		}
	}

	// Move the task directories
	for _, task := range tasks {
		otherTaskDir := filepath.Join(other.AllocDir, task.Name)
		otherTaskLocal := filepath.Join(otherTaskDir, TaskLocal)

		fileInfo, err := os.Stat(otherTaskLocal)
		if fileInfo != nil && err == nil {
			// TaskDirs haven't been built yet, so create it
			newTaskDir := filepath.Join(d.AllocDir, task.Name)
			if err := os.MkdirAll(newTaskDir, 0777); err != nil {
				return fmt.Errorf("error creating task %q dir: %v", task.Name, err)
			}
			localDir := filepath.Join(newTaskDir, TaskLocal)
			if err := os.Rename(otherTaskLocal, localDir); err != nil {
				return fmt.Errorf("error moving task %q local dir: %v", task.Name, err)
			}
		}
	}

	return nil
}

// Tears down previously build directory structure.
func (d *AllocDir) Destroy() error {

	// Unmount all mounted shared alloc dirs.
	var mErr multierror.Error
	if err := d.UnmountAll(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	if err := os.RemoveAll(d.AllocDir); err != nil {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to remove alloc dir %q: %v", d.AllocDir, err))
	}

	return mErr.ErrorOrNil()
}

// UnmountAll linked/mounted directories in task dirs.
func (d *AllocDir) UnmountAll() error {
	var mErr multierror.Error
	for _, dir := range d.TaskDirs {
		// Check if the directory has the shared alloc mounted.
		if pathExists(dir.SharedTaskDir) {
			if err := unlinkDir(dir.SharedTaskDir); err != nil {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("failed to unmount shared alloc dir %q: %v", dir.SharedTaskDir, err))
			} else if err := os.RemoveAll(dir.SharedTaskDir); err != nil {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("failed to delete shared alloc dir %q: %v", dir.SharedTaskDir, err))
			}
		}

		if pathExists(dir.SecretsDir) {
			if err := removeSecretDir(dir.SecretsDir); err != nil {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("failed to remove the secret dir %q: %v", dir.SecretsDir, err))
			}
		}

		// Unmount dev/ and proc/ have been mounted.
		if err := dir.unmountSpecialDirs(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Build the directory tree for an allocation.
func (d *AllocDir) Build() error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.MkdirAll(d.AllocDir, 0755); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %v", d.AllocDir, err)
	}

	// Make the shared directory and make it available to all user/groups.
	if err := os.MkdirAll(d.SharedDir, 0777); err != nil {
		return err
	}

	// Make the shared directory have non-root permissions.
	if err := dropDirPermissions(d.SharedDir); err != nil {
		return err
	}

	// Create shared subdirs
	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := os.MkdirAll(p, 0777); err != nil {
			return err
		}
		if err := dropDirPermissions(p); err != nil {
			return err
		}
	}

	return nil
}

// List returns the list of files at a path relative to the alloc dir
func (d *AllocDir) List(path string) ([]*AllocFileInfo, error) {
	if escapes, err := structs.PathEscapesAllocDir("", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)
	finfos, err := ioutil.ReadDir(p)
	if err != nil {
		return []*AllocFileInfo{}, err
	}
	files := make([]*AllocFileInfo, len(finfos))
	for idx, info := range finfos {
		files[idx] = &AllocFileInfo{
			Name:     info.Name(),
			IsDir:    info.IsDir(),
			Size:     info.Size(),
			FileMode: info.Mode().String(),
			ModTime:  info.ModTime(),
		}
	}
	return files, err
}

// Stat returns information about the file at a path relative to the alloc dir
func (d *AllocDir) Stat(path string) (*AllocFileInfo, error) {
	if escapes, err := structs.PathEscapesAllocDir("", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	return &AllocFileInfo{
		Size:     info.Size(),
		Name:     info.Name(),
		IsDir:    info.IsDir(),
		FileMode: info.Mode().String(),
		ModTime:  info.ModTime(),
	}, nil
}

// ReadAt returns a reader for a file at the path relative to the alloc dir
func (d *AllocDir) ReadAt(path string, offset int64) (io.ReadCloser, error) {
	if escapes, err := structs.PathEscapesAllocDir("", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	p := filepath.Join(d.AllocDir, path)

	// Check if it is trying to read into a secret directory
	for _, dir := range d.TaskDirs {
		if filepath.HasPrefix(p, dir.SecretsDir) {
			return nil, fmt.Errorf("Reading secret file prohibited: %s", path)
		}
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("can't seek to offset %q: %v", offset, err)
	}
	return f, nil
}

// BlockUntilExists blocks until the passed file relative the allocation
// directory exists. The block can be cancelled with the passed tomb.
func (d *AllocDir) BlockUntilExists(path string, t *tomb.Tomb) (chan error, error) {
	if escapes, err := structs.PathEscapesAllocDir("", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	// Get the path relative to the alloc directory
	p := filepath.Join(d.AllocDir, path)
	watcher := getFileWatcher(p)
	returnCh := make(chan error, 1)
	go func() {
		returnCh <- watcher.BlockUntilExists(t)
		close(returnCh)
	}()
	return returnCh, nil
}

// ChangeEvents watches for changes to the passed path relative to the
// allocation directory. The offset should be the last read offset. The tomb is
// used to clean up the watch.
func (d *AllocDir) ChangeEvents(path string, curOffset int64, t *tomb.Tomb) (*watch.FileChanges, error) {
	if escapes, err := structs.PathEscapesAllocDir("", path); err != nil {
		return nil, fmt.Errorf("Failed to check if path escapes alloc directory: %v", err)
	} else if escapes {
		return nil, fmt.Errorf("Path escapes the alloc directory")
	}

	// Get the path relative to the alloc directory
	p := filepath.Join(d.AllocDir, path)
	watcher := getFileWatcher(p)
	return watcher.ChangeEvents(t, curOffset)
}

// getFileWatcher returns a FileWatcher for the given path.
func getFileWatcher(path string) watch.FileWatcher {
	return watch.NewPollingFileWatcher(path)
}

func fileCopy(src, dst string, perm os.FileMode) error {
	// Do a simple copy.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Couldn't open src file %v: %v", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("Couldn't create destination file %v: %v", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Couldn't copy %v to %v: %v", src, dst, err)
	}

	return nil
}

// pathExists is a helper function to check if the path exists.
func pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// createDir creates a directory structure inside the basepath. This functions
// preserves the permissions of each of the subdirectories in the relative path
// by looking up the permissions in the host.
func createDir(basePath, relPath string) error {
	filePerms, err := splitPath(relPath)
	if err != nil {
		return err
	}

	// We are going backwards since we create the root of the directory first
	// and then create the entire nested structure.
	for i := len(filePerms) - 1; i >= 0; i-- {
		fi := filePerms[i]
		destDir := filepath.Join(basePath, fi.Name)
		if err := os.MkdirAll(destDir, fi.Perm); err != nil {
			return err
		}
	}
	return nil
}

// fileInfo holds the path and the permissions of a file
type fileInfo struct {
	Name string
	Perm os.FileMode
}

// splitPath stats each subdirectory of a path. The first element of the array
// is the file passed to this function, and the last element is the root of the
// path.
func splitPath(path string) ([]fileInfo, error) {
	var mode os.FileMode
	i, err := os.Stat(path)

	// If the path is not present in the host then we respond with the most
	// flexible permission.
	if err != nil {
		mode = os.ModePerm
	} else {
		mode = i.Mode()
	}
	var dirs []fileInfo
	dirs = append(dirs, fileInfo{Name: path, Perm: mode})
	currentDir := path
	for {
		dir := filepath.Dir(filepath.Clean(currentDir))
		if dir == currentDir {
			break
		}

		// We try to find the permission of the file in the host. If the path is not
		// present in the host then we respond with the most flexible permission.
		i, err = os.Stat(dir)
		if err != nil {
			mode = os.ModePerm
		} else {
			mode = i.Mode()
		}
		dirs = append(dirs, fileInfo{Name: dir, Perm: mode})
		currentDir = dir
	}
	return dirs, nil
}
