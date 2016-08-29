package allocdir

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/tomb.v1"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
)

const (
	// The minimum frequency to use for disk monitoring.
	minCheckDiskInterval = 3 * time.Minute

	// The maximum frequency to use for disk monitoring.
	maxCheckDiskInterval = 15 * time.Second

	// The amount of time that maxCheckDiskInterval is always used after
	// starting the allocation. This prevents unbounded disk usage that would
	// otherwise be possible for a number of minutes if we started with the
	// minCheckDiskInterval.
	checkDiskMaxEnforcePeriod = 5 * time.Minute
)

var (
	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// Name of the directory where logs of Tasks are written
	LogDirName = "logs"

	// The set of directories that exist inside eache shared alloc directory.
	SharedAllocDirs = []string{LogDirName, "tmp", "data"}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"

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
	TaskDirs map[string]string

	// Size is the total consumed disk size of the shared directory in bytes
	size     int64
	sizeLock sync.RWMutex

	// The minimum frequency to use for disk monitoring.
	MinCheckDiskInterval time.Duration

	// The maximum frequency to use for disk monitoring.
	MaxCheckDiskInterval time.Duration

	// The amount of time that maxCheckDiskInterval is always used after
	// starting the allocation. This prevents unbounded disk usage that would
	// otherwise be possible for a number of minutes if we started with the
	// minCheckDiskInterval.
	CheckDiskMaxEnforcePeriod time.Duration

	// running reflects the state of the disk watcher process.
	running bool

	// watchCh signals that the alloc directory is being torn down and that
	// any monitoring on it should stop.
	watchCh chan struct{}

	// MaxSize represents the total amount of megabytes that the shared allocation
	// directory is allowed to consume.
	MaxSize int
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
	BlockUntilExists(path string, t *tomb.Tomb) chan error
	ChangeEvents(path string, curOffset int64, t *tomb.Tomb) (*watch.FileChanges, error)
}

// NewAllocDir initializes the AllocDir struct with allocDir as base path for
// the allocation directory and maxSize as the maximum allowed size in megabytes.
func NewAllocDir(allocDir string, maxSize int) *AllocDir {
	d := &AllocDir{
		AllocDir:                  allocDir,
		MaxCheckDiskInterval:      maxCheckDiskInterval,
		MinCheckDiskInterval:      minCheckDiskInterval,
		CheckDiskMaxEnforcePeriod: checkDiskMaxEnforcePeriod,
		TaskDirs:                  make(map[string]string),
		MaxSize:                   maxSize,
	}
	d.SharedDir = filepath.Join(d.AllocDir, SharedAllocName)
	return d
}

// Tears down previously build directory structure.
func (d *AllocDir) Destroy() error {

	// Unmount all mounted shared alloc dirs.
	var mErr multierror.Error
	if err := d.UnmountAll(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	if err := os.RemoveAll(d.AllocDir); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

func (d *AllocDir) UnmountAll() error {
	var mErr multierror.Error
	for _, dir := range d.TaskDirs {
		// Check if the directory has the shared alloc mounted.
		taskAlloc := filepath.Join(dir, SharedAllocName)
		if d.pathExists(taskAlloc) {
			if err := d.unmountSharedDir(taskAlloc); err != nil {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("failed to unmount shared alloc dir %q: %v", taskAlloc, err))
			} else if err := os.RemoveAll(taskAlloc); err != nil {
				mErr.Errors = append(mErr.Errors,
					fmt.Errorf("failed to delete shared alloc dir %q: %v", taskAlloc, err))
			}
		}

		// Unmount dev/ and proc/ have been mounted.
		d.unmountSpecialDirs(dir)
	}

	return mErr.ErrorOrNil()
}

// Given a list of a task build the correct alloc structure.
func (d *AllocDir) Build(tasks []*structs.Task) error {
	// Make the alloc directory, owned by the nomad process.
	if err := os.MkdirAll(d.AllocDir, 0755); err != nil {
		return fmt.Errorf("Failed to make the alloc directory %v: %v", d.AllocDir, err)
	}

	// Make the shared directory and make it available to all user/groups.
	if err := os.MkdirAll(d.SharedDir, 0777); err != nil {
		return err
	}

	// Make the shared directory have non-root permissions.
	if err := d.dropDirPermissions(d.SharedDir); err != nil {
		return err
	}

	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := os.MkdirAll(p, 0777); err != nil {
			return err
		}
		if err := d.dropDirPermissions(p); err != nil {
			return err
		}
	}

	// Make the task directories.
	for _, t := range tasks {
		taskDir := filepath.Join(d.AllocDir, t.Name)
		if err := os.MkdirAll(taskDir, 0777); err != nil {
			return err
		}

		// Make the task directory have non-root permissions.
		if err := d.dropDirPermissions(taskDir); err != nil {
			return err
		}

		// Create a local directory that each task can use.
		local := filepath.Join(taskDir, TaskLocal)
		if err := os.MkdirAll(local, 0777); err != nil {
			return err
		}

		if err := d.dropDirPermissions(local); err != nil {
			return err
		}

		d.TaskDirs[t.Name] = taskDir

		// Create the directories that should be in every task.
		for _, dir := range TaskDirs {
			local := filepath.Join(taskDir, dir)
			if err := os.MkdirAll(local, 0777); err != nil {
				return err
			}

			if err := d.dropDirPermissions(local); err != nil {
				return err
			}
		}
	}

	return nil
}

// Embed takes a mapping of absolute directory or file paths on the host to
// their intended, relative location within the task directory. Embed attempts
// hardlink and then defaults to copying. If the path exists on the host and
// can't be embedded an error is returned.
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
					// Symlinking twice
					if err.(*os.LinkError).Err.Error() != "file exists" {
						return fmt.Errorf("Couldn't create symlink: %v", err)
					}
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

	return nil
}

// LogDir returns the log dir in the current allocation directory
func (d *AllocDir) LogDir() string {
	return filepath.Join(d.AllocDir, SharedAllocName, LogDirName)
}

// List returns the list of files at a path relative to the alloc dir
func (d *AllocDir) List(path string) ([]*AllocFileInfo, error) {
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
	p := filepath.Join(d.AllocDir, path)
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
func (d *AllocDir) BlockUntilExists(path string, t *tomb.Tomb) chan error {
	// Get the path relative to the alloc directory
	p := filepath.Join(d.AllocDir, path)
	watcher := getFileWatcher(p)
	returnCh := make(chan error, 1)
	go func() {
		returnCh <- watcher.BlockUntilExists(t)
		close(returnCh)
	}()
	return returnCh
}

// ChangeEvents watches for changes to the passed path relative to the
// allocation directory. The offset should be the last read offset. The tomb is
// used to clean up the watch.
func (d *AllocDir) ChangeEvents(path string, curOffset int64, t *tomb.Tomb) (*watch.FileChanges, error) {
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

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return fmt.Errorf("Couldn't create destination file %v: %v", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("Couldn't copy %v to %v: %v", src, dst, err)
	}

	return nil
}

// pathExists is a helper function to check if the path exists.
func (d *AllocDir) pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// GetSize returns the size of the shared allocation directory.
func (d *AllocDir) GetSize() int64 {
	d.sizeLock.Lock()
	defer d.sizeLock.Unlock()

	return d.size
}

// setSize sets the size of the shared allocation directory.
func (d *AllocDir) setSize(size int64) {
	d.sizeLock.Lock()
	defer d.sizeLock.Unlock()

	d.size = size
}

// StartDiskWatcher periodically checks the disk space consumed by the shared
// allocation directory.
func (d *AllocDir) StartDiskWatcher() {
	start := time.Now()

	sync := time.NewTimer(d.MaxCheckDiskInterval)
	defer sync.Stop()

	d.running = true
	d.watchCh = make(chan struct{})

	for {
		select {
		case <-d.watchCh:
			return
		case <-sync.C:
			if err := d.syncDiskUsage(); err != nil {
				log.Printf("[WARN] client: failed to sync disk usage: %v", err)
			}
			// Calculate the disk ratio.
			diskRatio := float64(d.size) / float64(d.MaxSize*structs.BytesInMegabyte)

			// Exponentially decrease the interval when the disk ratio increases.
			nextInterval := time.Duration(int64(1.0/(0.1*math.Pow(diskRatio, 2))+5)) * time.Second

			// Use the maximum interval for the first five minutes or if the
			// disk ratio is sufficiently high. Also use the minimum check interval
			// if the disk ratio becomes low enough.
			if nextInterval < d.MaxCheckDiskInterval || time.Since(start) < d.CheckDiskMaxEnforcePeriod {
				nextInterval = d.MaxCheckDiskInterval
			} else if nextInterval > d.MinCheckDiskInterval {
				nextInterval = d.MinCheckDiskInterval
			}
			sync.Reset(nextInterval)
		}
	}
}

// StopDiskWatcher closes the watch channel which causes the disk monitoring to stop.
func (d *AllocDir) StopDiskWatcher() {
	if d.running {
		d.running = false
		close(d.watchCh)
	}
}

// syncDiskUsage walks the allocation directory recursively and
// calculates the total consumed disk space.
func (d *AllocDir) syncDiskUsage() error {
	var size int64
	err := filepath.Walk(d.AllocDir,
		func(path string, info os.FileInfo, err error) error {
			// Ignore paths that do not have a valid FileInfo object
			if err == nil {
				size += info.Size()
			}
			return nil
		})
	// Store the disk consumption
	d.setSize(size)
	return err
}
