package client

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func testLogger() *log.Logger {
	return prefixedTestLogger("")
}

func prefixedTestLogger(prefix string) *log.Logger {
	return log.New(os.Stderr, prefix, log.LstdFlags)
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
	return testTaskRunnerFromAlloc(restarts, mock.Alloc())
}

// Creates a mock task runner using the first task in the first task group of
// the passed allocation.
func testTaskRunnerFromAlloc(restarts bool, alloc *structs.Allocation) (*MockTaskStateUpdater, *TaskRunner) {
	logger := testLogger()
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockTaskStateUpdater{}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	// Initialize the port listing. This should be done by the offer process but
	// we have a mock so that doesn't happen.
	task.Resources.Networks[0].ReservedPorts = []structs.Port{{"", 80}}

	allocDir := allocdir.NewAllocDir(filepath.Join(conf.AllocDir, alloc.ID))
	allocDir.Build([]*structs.Task{task})

	ctx := driver.NewExecContext(allocDir, alloc.ID)
	tr := NewTaskRunner(logger, conf, upd.Update, ctx, alloc, task)
	if !restarts {
		tr.restartTracker = noRestartsTracker()
	}
	return upd, tr
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(false)
	tr.MarkReceived()
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 3 {
		t.Fatalf("should have 3 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", upd.events[0].Type, structs.TaskReceived)
	}

	if upd.events[1].Type != structs.TaskStarted {
		t.Fatalf("Second Event was %v; want %v", upd.events[1].Type, structs.TaskStarted)
	}

	if upd.events[2].Type != structs.TaskTerminated {
		t.Fatalf("Third Event was %v; want %v", upd.events[2].Type, structs.TaskTerminated)
	}
}

func TestTaskRunner_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, tr := testTaskRunner(true)
	tr.MarkReceived()
	defer tr.ctx.AllocDir.Destroy()

	// Change command to ensure we run for a bit
	tr.task.Config["command"] = "/bin/sleep"
	tr.task.Config["args"] = []string{"1000"}
	go tr.Run()

	testutil.WaitForResult(func() (bool, error) {
		if l := len(upd.events); l != 2 {
			return false, fmt.Errorf("Expect two events; got %v", l)
		}

		if upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", upd.events[0].Type, structs.TaskReceived)
		}

		if upd.events[1].Type != structs.TaskStarted {
			return false, fmt.Errorf("Second Event was %v; want %v", upd.events[1].Type, structs.TaskStarted)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Make sure we are collecting  afew stats
	time.Sleep(2 * time.Second)
	stats := tr.LatestResourceUsage()
	if len(stats.Pids) == 0 || stats.ResourceUsage == nil || stats.ResourceUsage.MemoryStats.RSS == 0 {
		t.Fatalf("expected task runner to have some stats")
	}

	// Begin the tear down
	tr.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 3 {
		t.Fatalf("should have 3 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[2].Type != structs.TaskKilled {
		t.Fatalf("Third Event was %v; want %v", upd.events[2].Type, structs.TaskKilled)
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
	tr2 := NewTaskRunner(tr.logger, tr.config, upd.Update,
		tr.ctx, tr.alloc, &structs.Task{Name: tr.task.Name})
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

func TestTaskRunner_Download_List(t *testing.T) {
	ctestutil.ExecCompatible(t)

	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("."))))
	defer ts.Close()

	// Create an allocation that has a task with a list of artifacts.
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	f1 := "task_runner_test.go"
	f2 := "task_runner.go"
	artifact1 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f1),
	}
	artifact2 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f2),
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact1, &artifact2}

	upd, tr := testTaskRunnerFromAlloc(false, alloc)
	tr.MarkReceived()
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 4 {
		t.Fatalf("should have 4 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", upd.events[0].Type, structs.TaskReceived)
	}

	if upd.events[1].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Second Event was %v; want %v", upd.events[1].Type, structs.TaskDownloadingArtifacts)
	}

	if upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", upd.events[2].Type, structs.TaskStarted)
	}

	if upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Fourth Event was %v; want %v", upd.events[3].Type, structs.TaskTerminated)
	}

	// Check that both files exist.
	taskDir := tr.ctx.AllocDir.TaskDirs[task.Name]
	if _, err := os.Stat(filepath.Join(taskDir, f1)); err != nil {
		t.Fatalf("%v not downloaded", f1)
	}
	if _, err := os.Stat(filepath.Join(taskDir, f2)); err != nil {
		t.Fatalf("%v not downloaded", f2)
	}
}

func TestTaskRunner_Download_Retries(t *testing.T) {
	ctestutil.ExecCompatible(t)

	// Create an allocation that has a task with bad artifacts.
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	artifact := structs.TaskArtifact{
		GetterSource: "http://127.1.1.111:12315/foo/bar/baz",
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact}

	// Make the restart policy try one update
	alloc.Job.TaskGroups[0].RestartPolicy = &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    1 * time.Second,
		Mode:     structs.RestartPolicyModeFail,
	}

	upd, tr := testTaskRunnerFromAlloc(true, alloc)
	tr.MarkReceived()
	go tr.Run()
	defer tr.Destroy()
	defer tr.ctx.AllocDir.Destroy()

	select {
	case <-tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(upd.events) != 7 {
		t.Fatalf("should have 7 updates: %#v", upd.events)
	}

	if upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", upd.state, structs.TaskStateDead)
	}

	if upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", upd.events[0].Type, structs.TaskReceived)
	}

	if upd.events[1].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Second Event was %v; want %v", upd.events[1].Type, structs.TaskDownloadingArtifacts)
	}

	if upd.events[2].Type != structs.TaskArtifactDownloadFailed {
		t.Fatalf("Third Event was %v; want %v", upd.events[2].Type, structs.TaskArtifactDownloadFailed)
	}

	if upd.events[3].Type != structs.TaskRestarting {
		t.Fatalf("Fourth Event was %v; want %v", upd.events[3].Type, structs.TaskRestarting)
	}

	if upd.events[4].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Fifth Event was %v; want %v", upd.events[4].Type, structs.TaskDownloadingArtifacts)
	}

	if upd.events[5].Type != structs.TaskArtifactDownloadFailed {
		t.Fatalf("Sixth Event was %v; want %v", upd.events[5].Type, structs.TaskArtifactDownloadFailed)
	}

	if upd.events[6].Type != structs.TaskNotRestarting {
		t.Fatalf("Seventh Event was %v; want %v", upd.events[6].Type, structs.TaskNotRestarting)
	}
}

func TestTaskRunner_Validate_UserEnforcement(t *testing.T) {
	_, tr := testTaskRunner(false)

	// Try to run as root with exec.
	tr.task.Driver = "exec"
	tr.task.User = "root"
	if err := tr.validateTask(); err == nil {
		t.Fatalf("expected error running as root with exec")
	}

	// Try to run a non-blacklisted user with exec.
	tr.task.Driver = "exec"
	tr.task.User = "foobar"
	if err := tr.validateTask(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to run as root with docker.
	tr.task.Driver = "docker"
	tr.task.User = "root"
	if err := tr.validateTask(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
