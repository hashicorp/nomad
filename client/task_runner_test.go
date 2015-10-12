package client

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/discovery"
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
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockTaskStateUpdater{}
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	// Initialize the port listing. This should be done by the offer process but
	// we have a mock so that doesn't happen.
	task.Resources.Networks[0].ReservedPorts = []int{80}

	allocDir := allocdir.NewAllocDir(filepath.Join(conf.AllocDir, alloc.ID))
	allocDir.Build([]*structs.Task{task})

	ctx := driver.NewExecContext(allocDir)
	tr := NewTaskRunner(logger, conf, upd.Update, ctx, alloc.ID, alloc.JobID,
		alloc.TaskGroup, task, nil)
	return upd, tr
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner()
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

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
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner()
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
	ctestutil.ExecCompatible(t)
	_, tr := testTaskRunner()

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

/*
TODO: This test is disabled til a follow-up api changes the restore state interface.
The driver/executor interface will be changed from Open to Cleanup, in which
clean-up tears down previous allocs.

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
*/

func TestTaskRunner_HandleDiscovery(t *testing.T) {
	// Create the task runner
	_, tr := testTaskRunner()

	// Modify the job so we have predictable results
	tr.allocID = "1"
	tr.jobID = "job1"
	tr.taskGroup = "group1"
	tr.task.Name = "web"

	// Assign dynamic ports. The ReservedPorts are overloaded for dynamic
	// ports also, so we set those here as well.
	tr.task.Resources.Networks[0].DynamicPorts = []string{"http", "https"}
	tr.task.Resources.Networks[0].ReservedPorts = []int{80, 443}

	// Create the mock discovery layer
	providers := []discovery.Factory{discovery.NewMockDiscovery}
	disc, err := discovery.NewDiscoveryLayer(providers, tr.config, tr.logger, tr.config.Node)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	tr.discovery = disc
	mp := disc.Providers[0].(*discovery.MockDiscovery)

	// Register the task with discovery
	tr.handleDiscovery(false)

	// Check the result
	expect := map[string]int{
		"job1.group1.web:1":       0,
		"job1.group1.web.http:1":  80,
		"job1.group1.web.https:1": 443,
	}
	if !reflect.DeepEqual(expect, mp.Registered) {
		t.Fatalf("bad: %#v", mp.Registered)
	}

	// Deregister
	tr.handleDiscovery(true)
	if len(mp.Registered) != 0 {
		t.Fatalf("bad: %#v", mp.Registered)
	}

	// Assign a discover name to the task. Should replace the long prefix.
	tr.task.Discover = "web"
	tr.handleDiscovery(false)
	expect = map[string]int{
		"web:1":       0,
		"web.http:1":  80,
		"web.https:1": 443,
	}
	if !reflect.DeepEqual(expect, mp.Registered) {
		t.Fatalf("bad: %#v", mp.Registered)
	}

	// Deregister
	tr.handleDiscovery(true)
	if len(mp.Registered) != 0 {
		t.Fatalf("bad: %#v", mp.Registered)
	}
}
