package client

import (
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

type MockTaskStateUpdater struct {
	Count       int
	Name        []string
	Status      []string
	Description []string
}

func (m *MockTaskStateUpdater) Update(name, status, desc string) {
	m.Count += 1
	m.Name = append(m.Name, name)
	m.Status = append(m.Status, status)
	m.Description = append(m.Description, desc)
}

func testTaskRunner() (*MockTaskStateUpdater, *TaskRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = "/tmp"
	upd := &MockTaskStateUpdater{}
	ctx := driver.NewExecContext()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	tr := NewTaskRunner(logger, conf, upd.Update, ctx, alloc.ID, task)
	return upd, tr
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	upd, tr := testTaskRunner()
	go tr.Run()
	defer tr.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}

	if upd.Count != 2 {
		t.Fatalf("should have 2 updates: %#v", upd)
	}
	if upd.Name[0] != tr.task.Name {
		t.Fatalf("bad: %#v", upd.Name)
	}
	if upd.Status[0] != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", upd.Status)
	}
	if upd.Description[0] != "task started" {
		t.Fatalf("bad: %#v", upd.Description)
	}

	if upd.Name[1] != tr.task.Name {
		t.Fatalf("bad: %#v", upd.Name)
	}
	if upd.Status[1] != structs.AllocClientStatusDead {
		t.Fatalf("bad: %#v", upd.Status)
	}
	if upd.Description[1] != "task completed" {
		t.Fatalf("bad: %#v", upd.Description)
	}
}

func TestTaskRunner_Destroy(t *testing.T) {
	upd, tr := testTaskRunner()

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
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}

	if upd.Count != 2 {
		t.Fatalf("should have 2 updates: %#v", upd)
	}
	if upd.Status[0] != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", upd.Status)
	}
	if upd.Status[1] != structs.AllocClientStatusDead {
		t.Fatalf("bad: %#v", upd.Status)
	}
	if !strings.Contains(upd.Description[1], "task failed") {
		t.Fatalf("bad: %#v", upd.Description)
	}
}

func TestTaskRunner_Update(t *testing.T) {
	_, tr := testTaskRunner()

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = "10"
	go tr.Run()
	defer tr.Destroy()

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
	upd, tr := testTaskRunner()

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = "10"
	go tr.Run()
	defer tr.Destroy()

	// Snapshot state
	time.Sleep(200 * time.Millisecond)
	err := tr.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new task runner
	tr2 := NewTaskRunner(tr.logger, tr.config, upd.Update,
		tr.ctx, tr.allocID, &structs.Task{Name: tr.task.Name})
	err = tr2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go tr2.Run()
	defer tr2.Destroy()

	// Destroy and wait
	tr2.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}
