package proclib

import (
	"github.com/shoenig/netlog"

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

	lock sync.Mutex
	m    map[Task]ProcessWrangler
}

func (w *Wranglers) Setup(task Task) error {
	w.configs.Logger.Info("Setup() hi", "task", task)

	// since we do not know the cgroup in the alloc/task runner hook, we need
	// to be able to modify the process wrangler in a post start hook? or do
	// not call this until post start?

	// create process wrangler for task
	pw := w.create(task)

	pw.Initialize()

	w.lock.Lock()
	defer w.lock.Unlock()

	// keep track of the process wrangler for task
	netlog.Green("Wranglers.Setup()", "task", task, "pw", pw)
	w.m[task] = pw

	return nil
}

func (w *Wranglers) Destroy(task Task) error {
	w.configs.Logger.Info("Destroy()", "task", task)

	w.lock.Lock()
	defer w.lock.Unlock()

	netlog.Yellow("Wranglers.Destroy", "task", task, "w", w)
	netlog.Yellow("C", "w.m", w.m)
	netlog.Yellow("C", "w.m[task]", w.m[task])

	w.m[task].Kill()
	w.m[task].Cleanup()

	delete(w.m, task)

	return nil
}

// A ProcessWrangler is initialized (in AR?) and used to manage the thing
// used to manage the underlying process of a Task (e.g. cgroups).
type ProcessWrangler interface {
	Initialize() error
	Kill() error
	Cleanup() error
}
