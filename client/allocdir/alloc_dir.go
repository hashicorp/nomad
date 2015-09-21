package allocdir

import (
	"fmt"
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

// Builds the necessary directory structure for running an alloc.
type AllocDirBuilder interface {
	// Given a list of a task build the correct alloc structure.
	Build([]*structs.Task) error

	// Tears down previously build directory structure.
	Destroy() error

	// Returns the directory of a task if it was created, otherwise an error is
	// returned.
	TaskDir(task string) (string, error)
}

type AllocDir struct {
	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string

	// The shared directory is available to all tasks within the same task
	// group.
	SharedDir string

	// TaskDirs is a mapping of task names to their non-shared directory.
	TaskDirs map[string]string
}

func NewAllocDir(allocDir string) *AllocDir {
	d := &AllocDir{AllocDir: allocDir, TaskDirs: make(map[string]string)}
	d.SharedDir = filepath.Join(d.AllocDir, SharedAllocName)
	return d
}

func (d *AllocDir) Destroy() error {
	return os.RemoveAll(d.AllocDir)
}

func (d *AllocDir) TaskDir(task string) (string, error) {
	if dir, ok := d.TaskDirs[task]; ok {
		return dir, nil
	}

	return "", fmt.Errorf("Task directory doesn't exist for task %v", task)
}
