// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"context"
	"io"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
)

// AllocDir2 enables creating, destroying, and accessing an allocation's
// directory making using of the alloc2 directory structure. The alloc2 directory
// structure is such that the user of each task is the owner of its own task
// directory. This is contrast with the original alloc directory structure
// where each task directory is owned by nobody with mode 0777.
//
// The goal is to enable task drivers like exec2, and in the future raw_exec
// and pledge to run tasks as non-root users.
//
// All methods are safe for concurrent use.
type AllocDir2 struct {
	lock sync.RWMutex

	// the directory used for storing state of this allocation
	allocDir string

	// shareDir is available to each tasks within the same group.
	shareDir string

	taskDirs map[string]*TaskDir

	// rootDir is the directory in which all other directories in the alloc2
	// directory structure are created. In the alloc2 directory structure this
	// directory is owned by the nomad user with mode 0755.
	rootDir string

	// logger for these things
	logger hclog.Logger
}

func NewAllocDir2(logger hclog.Logger, rootDir, allocID string) *AllocDir2 {
	return &AllocDir2{
		rootDir: rootDir,
	}
}

func (a *AllocDir2) WalkTaskDirs(f func(*TaskDir) error) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	for _, taskDir := range a.taskDirs {
		if err := f(taskDir); err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocDir2) AllocDirPath() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.allocDir
}

func (a *AllocDir2) ShareDirPath() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.shareDir
}

func (a *AllocDir2) GetTaskDir(task string) *TaskDir {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.taskDirs[task]
}

func (a *AllocDir2) Build() error {
	panic("not yet implemented")
}

func (a *AllocDir2) Move(Interface, []*structs.Task) error {
	panic("not yet implemented")
}

func (a *AllocDir2) Destroy() error {
	err := destroy(a)
	return err
}

func (a *AllocDir2) BlockUntilExists(ctx context.Context, path string) (chan error, error) {
	return blockUntilExists(a, ctx, path)
}

func (a *AllocDir2) ChangeEvents(ctx context.Context, path string, offset int64) (*watch.FileChanges, error) {
	return changeEvents(a, ctx, path, offset)
}

func (a *AllocDir2) NewTaskDir(task string) *TaskDir {
	panic("not yet implemented")
}

func (a *AllocDir2) List(path string) ([]*cstructs.AllocFileInfo, error) {
	return list(a, path)
}

func (a *AllocDir2) Stat(path string) (*cstructs.AllocFileInfo, error) {
	return stat(a, path)
}

func (a *AllocDir2) ReadAt(path string, offset int64) (io.ReadCloser, error) {
	return readAt(a, path, offset)
}

func (a *AllocDir2) Snapshot(w io.Writer) error {
	allocDir := a.AllocDirPath()
	shareDir := a.ShareDirPath()

	allocDataDir := filepath.Join(shareDir, SharedDataDir)
	rootPaths := []string{allocDataDir}
	_ = a.WalkTaskDirs(func(taskdir *TaskDir) error {
		rootPaths = append(rootPaths, taskdir.LocalDir)
		return nil
	})

	return snapshot(a, allocDir, shareDir, rootPaths, w)
}
