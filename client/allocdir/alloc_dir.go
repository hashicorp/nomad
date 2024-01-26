// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
)

const (
	// idUnsupported is what the uid/gid will be set to on platforms (eg
	// Windows) that don't support integer ownership identifiers.
	idUnsupported = -1
)

var (
	// SnapshotErrorTime is the sentinel time that will be used on the
	// error file written by Snapshot when it encounters as error.
	SnapshotErrorTime = time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)

	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// Name of the directory where logs of Tasks are written
	LogDirName = "logs"

	// SharedDataDir is one of the shared allocation directories. It is
	// included in snapshots.
	SharedDataDir = "data"

	// TmpDirName is the name of the temporary directory in each alloc and
	// task.
	TmpDirName = "tmp"

	// The set of directories that exist inside each shared alloc directory.
	SharedAllocDirs = []string{LogDirName, TmpDirName, SharedDataDir}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"

	// TaskSecrets is the name of the secret directory inside each task
	// directory
	TaskSecrets = "secrets"

	// TaskPrivate is the name of the private directory inside each task
	// directory
	TaskPrivate = "private"

	// TaskDirs is the set of directories created in each tasks directory.
	TaskDirs = map[string]os.FileMode{TmpDirName: os.ModeSticky | 0777}

	// AllocGRPCSocket is the path relative to the task dir root for the
	// unix socket connected to Consul's gRPC endpoint.
	AllocGRPCSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_grpc.sock")

	// AllocHTTPSocket is the path relative to the task dir root for the unix
	// socket connected to Consul's HTTP endpoint.
	AllocHTTPSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_http.sock")
)

// AllocDir allows creating, destroying, and accessing an allocation's
// directory.
//
// All methods are safe for concurrent use.
type AllocDir struct {
	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string

	// The shared directory is available to all tasks within the same task
	// group.
	SharedDir string

	// TaskDirs is a mapping of task names to their non-shared directory.
	TaskDirs map[string]*TaskDir

	// clientAllocDir is the client agent's root alloc directory. It must
	// be excluded from chroots and is configured via client.alloc_dir.
	clientAllocDir string

	// built is true if Build has successfully run
	built bool

	mu sync.RWMutex

	logger hclog.Logger
}

func (a *AllocDir) AllocDirPath() string {
	return a.AllocDir
}

func (a *AllocDir) ShareDirPath() string {
	return a.SharedDir
}

func (a *AllocDir) GetTaskDir(task string) *TaskDir {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.TaskDirs[task]
}

// NewAllocDir initializes the AllocDir struct with allocDir as base path for
// the allocation directory.
func NewAllocDir(logger hclog.Logger, clientAllocDir, allocID string) *AllocDir {
	logger = logger.Named("alloc_dir")
	allocDir := filepath.Join(clientAllocDir, allocID)
	shareDir := filepath.Join(allocDir, SharedAllocName)

	return &AllocDir{
		clientAllocDir: clientAllocDir,
		AllocDir:       allocDir,
		SharedDir:      shareDir,
		TaskDirs:       make(map[string]*TaskDir),
		logger:         logger,
	}
}

// NewTaskDir creates a new TaskDir and adds it to the AllocDirs TaskDirs map.
func (d *AllocDir) NewTaskDir(name string) *TaskDir {
	d.mu.Lock()
	defer d.mu.Unlock()

	td := newTaskDir(d.logger, d.clientAllocDir, d.AllocDir, name)
	d.TaskDirs[name] = td
	return td
}

// Move other alloc directory's shared path and local dir to this alloc dir.
func (d *AllocDir) Move(other Interface, tasks []*structs.Task) error {
	d.mu.RLock()
	if !d.built {
		// Enforce the invariant that Build is called before Move
		d.mu.RUnlock()
		return fmt.Errorf("unable to move to %q - alloc dir is not built", d.AllocDir)
	}

	// Moving is slow and only reads immutable fields, so unlock during heavy IO
	d.mu.RUnlock()

	// Move the data directory
	otherDataDir := filepath.Join(other.ShareDirPath(), SharedDataDir)
	dataDir := filepath.Join(d.SharedDir, SharedDataDir)
	if fileInfo, err := os.Stat(otherDataDir); fileInfo != nil && err == nil {
		os.Remove(dataDir) // remove an empty data dir if it exists
		if err := os.Rename(otherDataDir, dataDir); err != nil {
			return fmt.Errorf("error moving data dir: %v", err)
		}
	}

	// Move the task directories
	for _, task := range tasks {
		otherTaskDir := filepath.Join(other.AllocDirPath(), task.Name)
		otherTaskLocal := filepath.Join(otherTaskDir, TaskLocal)

		fileInfo, err := os.Stat(otherTaskLocal)
		if fileInfo != nil && err == nil {
			// TaskDirs haven't been built yet, so create it
			newTaskDir := filepath.Join(d.AllocDir, task.Name)
			if err := os.MkdirAll(newTaskDir, 0777); err != nil {
				return fmt.Errorf("error creating task %q dir: %v", task.Name, err)
			}
			localDir := filepath.Join(newTaskDir, TaskLocal)
			os.Remove(localDir) // remove an empty local dir if it exists
			if err := os.Rename(otherTaskLocal, localDir); err != nil {
				return fmt.Errorf("error moving task %q local dir: %v", task.Name, err)
			}
		}
	}

	return nil
}

// Destroy tears down previously build directory structure.
func (d *AllocDir) Destroy() error {
	err := destroy(d)

	d.mu.Lock()
	d.built = false
	d.mu.Unlock()
	return err
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
	if err := dropDirPermissions(d.SharedDir, os.ModePerm); err != nil {
		return err
	}

	// Create shared subdirs
	for _, dir := range SharedAllocDirs {
		p := filepath.Join(d.SharedDir, dir)
		if err := os.MkdirAll(p, 0777); err != nil {
			return err
		}
		if err := dropDirPermissions(p, os.ModePerm); err != nil {
			return err
		}
	}

	// Mark as built
	d.mu.Lock()
	d.built = true
	d.mu.Unlock()
	return nil
}

func (d *AllocDir) WalkTaskDirs(f func(*TaskDir) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, taskDir := range d.TaskDirs {
		if err := f(taskDir); err != nil {
			return err
		}
	}
	return nil
}

// List returns the list of files at a path relative to the alloc dir
func (d *AllocDir) List(path string) ([]*cstructs.AllocFileInfo, error) {
	return list(d, path)
}

// Stat returns information about the file at a path relative to the alloc dir
func (d *AllocDir) Stat(path string) (*cstructs.AllocFileInfo, error) {
	return stat(d, path)
}

// ReadAt returns a reader for a file at the path relative to the alloc dir
func (d *AllocDir) ReadAt(path string, offset int64) (io.ReadCloser, error) {
	return readAt(d, path, offset)
}

// BlockUntilExists blocks until the passed file relative the allocation
// directory exists. The block can be cancelled with the passed context.
func (d *AllocDir) BlockUntilExists(ctx context.Context, path string) (chan error, error) {
	return blockUntilExists(d, ctx, path)
}

// ChangeEvents watches for changes to the passed path relative to the
// allocation directory. The offset should be the last read offset. The context is
// used to clean up the watch.
func (d *AllocDir) ChangeEvents(ctx context.Context, path string, curOffset int64) (*watch.FileChanges, error) {
	return changeEvents(d, ctx, path, curOffset)
}

// Snapshot creates an archive of the files and directories in the data dir of
// the allocation and the task local directories
//
// Since a valid tar may have been written even when an error occurs, a special
// file "NOMAD-${ALLOC_ID}-ERROR.log" will be appended to the tar with the
// error message as the contents.
func (d *AllocDir) Snapshot(w io.Writer) error {
	allocDir := d.AllocDirPath()
	shareDir := d.ShareDirPath()

	allocDataDir := filepath.Join(shareDir, SharedDataDir)
	rootPaths := []string{allocDataDir}
	_ = d.WalkTaskDirs(func(taskdir *TaskDir) error {
		rootPaths = append(rootPaths, taskdir.LocalDir)
		return nil
	})

	return snapshot(d, allocDir, shareDir, rootPaths, w)
}
