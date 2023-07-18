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

type Wranglers struct {
	lock sync.Mutex
	m    map[Task]ProcessWrangler

	create create
}

type create func(Task) ProcessWrangler

// A ProcessWrangler is initialized (in AR?) and used to manage the thing
// used to manage the underlying process of a Task (e.g. cgroups).
type ProcessWrangler interface {
	Kill() error
	Cleanup() error
}
