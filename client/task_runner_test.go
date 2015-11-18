package client

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

type MockTaskStateUpdater struct{}

func (m *MockTaskStateUpdater) Update(name string) {}

func testTaskRunner(restarts bool) (*MockTaskStateUpdater, *TaskRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockTaskStateUpdater{}
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	consulClient, _ := NewConsulClient(logger)
	// Initialize the port listing. This should be done by the offer process but
	// we have a mock so that doesn't happen.
	task.Resources.Networks[0].ReservedPorts = []structs.Port{{"", 80}}

	allocDir := allocdir.NewAllocDir(filepath.Join(conf.AllocDir, alloc.ID))
	allocDir.Build([]*structs.Task{task})

	ctx := driver.NewExecContext(allocDir, alloc.ID)
	rp := structs.NewRestartPolicy(structs.JobTypeService)
	restartTracker := newRestartTracker(structs.JobTypeService, rp)
	if !restarts {
		restartTracker = noRestartsTracker()
	}

	state := alloc.TaskStates[task.Name]
	tr := NewTaskRunner(logger, conf, upd.Update, ctx, alloc.ID, task, state, restartTracker, consulClient)
	return upd, tr
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	_, tr := testTaskRunner(false)
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}

	if len(tr.state.Events) != 2 {
		t.Fatalf("should have 2 updates: %#v", tr.state.Events)
	}

	if tr.state.State != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", tr.state.State, structs.TaskStateDead)
	}

	if tr.state.Events[0].Type != structs.TaskStarted {
		t.Fatalf("First Event was %v; want %v", tr.state.Events[0].Type, structs.TaskStarted)
	}

	if tr.state.Events[1].Type != structs.TaskTerminated {
		t.Fatalf("First Event was %v; want %v", tr.state.Events[1].Type, structs.TaskTerminated)
	}
}

func TestTaskRunner_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	_, tr := testTaskRunner(true)
	defer tr.ctx.AllocDir.Destroy()

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = "10"
	go tr.Run()

	// Begin the tear down
	go func() {
		time.Sleep(100 * time.Millisecond)
		tr.Destroy()
	}()

	select {
	case <-tr.WaitCh():
	case <-time.After(8 * time.Second):
		t.Fatalf("timeout")
	}

	if len(tr.state.Events) != 2 {
		t.Fatalf("should have 2 updates: %#v", tr.state.Events)
	}

	if tr.state.State != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", tr.state.State, structs.TaskStateDead)
	}

	if tr.state.Events[0].Type != structs.TaskStarted {
		t.Fatalf("First Event was %v; want %v", tr.state.Events[0].Type, structs.TaskStarted)
	}

	if tr.state.Events[1].Type != structs.TaskKilled {
		t.Fatalf("First Event was %v; want %v", tr.state.Events[1].Type, structs.TaskKilled)
	}

}

func TestTaskRunner_Update(t *testing.T) {
	ctestutil.ExecCompatible(t)
	_, tr := testTaskRunner(false)

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = "10"
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	// Update the task definition
	newTask := new(structs.Task)
	*newTask = *tr.task
	newTask.Driver = "foobar"
	tr.Update(newTask)

	// Wait for update to take place
	testutil.WaitForResult(func() (bool, error) {
		return tr.task == newTask, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_SaveRestoreState(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(false)

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = "10"
	go tr.Run()
	defer tr.Destroy()

	// Snapshot state
	time.Sleep(1 * time.Second)
	if err := tr.SaveState(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new task runner
	consulClient, _ := NewConsulClient(tr.logger)
	tr2 := NewTaskRunner(tr.logger, tr.config, upd.Update,
		tr.ctx, tr.allocID, &structs.Task{Name: tr.task.Name}, tr.state, tr.restartTracker,
		consulClient)
	if err := tr2.RestoreState(); err != nil {
		t.Fatalf("err: %v", err)
	}
	go tr2.Run()
	defer tr2.Destroy()

	// Destroy and wait
	time.Sleep(1 * time.Second)
	if tr2.handle == nil {
		t.Fatalf("RestoreState() didn't open handle")
	}
}
