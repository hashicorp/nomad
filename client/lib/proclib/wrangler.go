package proclib

import (
	"fmt"
	"sync"
)

type Task struct {
	AllocID string
	Task    string
}

func (task Task) String() string {
	return fmt.Sprintf("%s/%s", task.AllocID[0:8], task.Task)
}

type create func(Task) ProcessWrangler

type Wranglers struct {
	configs *Configs
	create  create
	lock    sync.Mutex
	m       map[Task]ProcessWrangler
}

func (w *Wranglers) Add(task Task) {
	nlog.Trace("Wranglers.Add()", "task", task)

	// since we do not know the cgroup in the alloc/task runner hook, we need
	// to be able to modify the process wrangler in a post start hook? or do
	// not call this until post start?

	// create process wrangler for task
	pw := w.create(task)

	w.lock.Lock()
	defer w.lock.Unlock()

	// keep track of the process wrangler for task
	w.m[task] = pw
}

// A ProcessWrangler is initialized (in AR?) and used to manage the thing
// used to manage the underlying process of a Task (e.g. cgroups).
type ProcessWrangler interface {
	Kill() error
	Cleanup() error
}
