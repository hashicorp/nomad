package client

import (
	"fmt"
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

type MockTaskStateUpdater struct {
	state  string
	events []*structs.TaskEvent
}

func (m *MockTaskStateUpdater) Update(name, state string, event *structs.TaskEvent) {
	m.state = state
	m.events = append(m.events, event)
}

func testTaskRunner(restarts bool) (*MockTaskStateUpdater, *TaskRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockTaskStateUpdater{}
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	consulClient, _ := NewConsulService(&consulServiceConfig{logger, "127.0.0.1:8500", "", "", false, false, &structs.Node{}})
	// Initialize the port listing. This should be done by the offer process but
	// we have a mock so that doesn't happen.
	task.Resources.Networks[0].ReservedPorts = []structs.Port{{"", 80}}

	allocDir := allocdir.NewAllocDir(filepath.Join(conf.AllocDir, alloc.ID))
	allocDir.Build([]*structs.Task{task})

	ctx := driver.NewExecContext(allocDir, alloc.ID)
	tr := NewTaskRunner(logger, conf, upd.Update, ctx, mock.Alloc(), task, consulClient)
	if !restarts {
		tr.restartTracker = noRestartsTracker()
	}
	return upd, tr
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(false)
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 2 {
		t.Fatalf("should have 2 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[0].Type != structs.TaskStarted {
		t.Fatalf("First Event was %v; want %v", upd.events[0].Type, structs.TaskStarted)
	}

	if upd.events[1].Type != structs.TaskTerminated {
		t.Fatalf("First Event was %v; want %v", upd.events[1].Type, structs.TaskTerminated)
	}
}

func TestTaskRunner_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(true)
	defer tr.ctx.AllocDir.Destroy()

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = []string{"10"}
	go tr.Run()

	// Begin the tear down
	go func() {
		time.Sleep(100 * time.Millisecond)
		tr.Destroy()
	}()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 2 {
		t.Fatalf("should have 2 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[0].Type != structs.TaskStarted {
		t.Fatalf("First Event was %v; want %v", upd.events[0].Type, structs.TaskStarted)
	}

	if upd.events[1].Type != structs.TaskKilled {
		t.Fatalf("First Event was %v; want %v", upd.events[1].Type, structs.TaskKilled)
	}

}

func TestTaskRunner_Update(t *testing.T) {
	ctestutil.ExecCompatible(t)
	_, tr := testTaskRunner(false)

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = []string{"100"}
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	// Update the task definition
	updateAlloc := tr.alloc.Copy()

	// Update the restart policy
	newTG := updateAlloc.Job.TaskGroups[0]
	newMode := "foo"
	newTG.RestartPolicy.Mode = newMode

	newTask := updateAlloc.Job.TaskGroups[0].Tasks[0]
	newTask.Driver = "foobar"

	// Update the kill timeout
	testutil.WaitForResult(func() (bool, error) {
		if tr.handle == nil {
			return false, fmt.Errorf("task not started")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	oldHandle := tr.handle.ID()
	newTask.KillTimeout = time.Hour

	tr.Update(updateAlloc)

	// Wait for update to take place
	testutil.WaitForResult(func() (bool, error) {
		if tr.task != newTask {
			return false, fmt.Errorf("task not updated")
		}
		if tr.restartTracker.policy.Mode != newMode {
			return false, fmt.Errorf("restart policy not updated")
		}
		if tr.handle.ID() == oldHandle {
			return false, fmt.Errorf("handle not updated")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_SaveRestoreState(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(false)

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = []string{"10"}
	go tr.Run()
	defer tr.Destroy()

	// Snapshot state
	time.Sleep(2 * time.Second)
	if err := tr.SaveState(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new task runner
	consulClient, _ := NewConsulService(&consulServiceConfig{tr.logger, "127.0.0.1:8500", "", "", false, false, &structs.Node{}})
	tr2 := NewTaskRunner(tr.logger, tr.config, upd.Update,
		tr.ctx, tr.alloc, &structs.Task{Name: tr.task.Name}, consulClient)
	if err := tr2.RestoreState(); err != nil {
		t.Fatalf("err: %v", err)
	}
	go tr2.Run()
	defer tr2.Destroy()

	// Destroy and wait
	testutil.WaitForResult(func() (bool, error) {
		return tr2.handle != nil, fmt.Errorf("RestoreState() didn't open handle")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
