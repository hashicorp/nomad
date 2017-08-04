package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func testLogger() *log.Logger {
	return prefixedTestLogger("")
}

func prefixedTestLogger(prefix string) *log.Logger {
	if testing.Verbose() {
		return log.New(os.Stderr, prefix, log.LstdFlags|log.Lmicroseconds)
	}
	return log.New(ioutil.Discard, "", 0)
}

type MockTaskStateUpdater struct {
	state  string
	failed bool
	events []*structs.TaskEvent
}

func (m *MockTaskStateUpdater) Update(name, state string, event *structs.TaskEvent) {
	if state != "" {
		m.state = state
	}
	if event != nil {
		if event.FailsTask {
			m.failed = true
		}
		m.events = append(m.events, event)
	}
}

type taskRunnerTestCtx struct {
	upd      *MockTaskStateUpdater
	tr       *TaskRunner
	allocDir *allocdir.AllocDir
}

// Cleanup calls Destroy on the task runner and alloc dir
func (ctx *taskRunnerTestCtx) Cleanup() {
	ctx.tr.Destroy(structs.NewTaskEvent(structs.TaskKilled))
	ctx.allocDir.Destroy()
}

func testTaskRunner(t *testing.T, restarts bool) *taskRunnerTestCtx {
	// Use mock driver
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "500ms"
	return testTaskRunnerFromAlloc(t, restarts, alloc)
}

// Creates a mock task runner using the first task in the first task group of
// the passed allocation.
//
// Callers should defer Cleanup() to cleanup after completion
func testTaskRunnerFromAlloc(t *testing.T, restarts bool, alloc *structs.Allocation) *taskRunnerTestCtx {
	logger := testLogger()
	conf := config.DefaultConfig()
	conf.Node = mock.Node()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()

	tmp, err := ioutil.TempFile("", "state-db")
	if err != nil {
		t.Fatalf("error creating state db file: %v", err)
	}
	db, err := bolt.Open(tmp.Name(), 0600, nil)
	if err != nil {
		t.Fatalf("error creating state db: %v", err)
	}

	upd := &MockTaskStateUpdater{}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	// Initialize the port listing. This should be done by the offer process but
	// we have a mock so that doesn't happen.
	task.Resources.Networks[0].ReservedPorts = []structs.Port{{Label: "", Value: 80}}

	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(conf.AllocDir, alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("error building alloc dir: %v", err)
		return nil
	}

	//HACK to get FSIsolation and chroot without using AllocRunner,
	//     TaskRunner, or Drivers
	fsi := cstructs.FSIsolationImage
	switch task.Driver {
	case "raw_exec":
		fsi = cstructs.FSIsolationNone
	case "exec", "java":
		fsi = cstructs.FSIsolationChroot
	}
	taskDir := allocDir.NewTaskDir(task.Name)
	if err := taskDir.Build(false, config.DefaultChrootEnv, fsi); err != nil {
		t.Fatalf("error building task dir %q: %v", task.Name, err)
		return nil
	}

	vclient := vaultclient.NewMockVaultClient()
	cclient := newMockConsulServiceClient()
	tr := NewTaskRunner(logger, conf, db, upd.Update, taskDir, alloc, task, vclient, cclient)
	if !restarts {
		tr.restartTracker = noRestartsTracker()
	}
	return &taskRunnerTestCtx{upd, tr, allocDir}
}

// testWaitForTaskToStart waits for the task to or fails the test
func testWaitForTaskToStart(t *testing.T, ctx *taskRunnerTestCtx) {
	// Wait for the task to start
	testutil.WaitForResult(func() (bool, error) {
		l := len(ctx.upd.events)
		if l < 2 {
			return false, fmt.Errorf("Expect two events; got %v", l)
		}

		if ctx.upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
		}

		if l >= 3 {
			if ctx.upd.events[1].Type != structs.TaskSetup {
				return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
			}
			if ctx.upd.events[2].Type != structs.TaskStarted {
				return false, fmt.Errorf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
			}
		} else {
			if ctx.upd.events[1].Type != structs.TaskStarted {
				return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskStarted)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_SimpleRun(t *testing.T) {
	t.Parallel()
	ctx := testTaskRunner(t, false)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 4 {
		t.Fatalf("should have 3 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskTerminated)
	}
}

func TestTaskRunner_Run_RecoverableStartError(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code":               0,
		"start_error":             "driver failure",
		"start_error_recoverable": true,
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	testutil.WaitForResult(func() (bool, error) {
		if l := len(ctx.upd.events); l < 4 {
			return false, fmt.Errorf("Expect at least four events; got %v", l)
		}

		if ctx.upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
		}

		if ctx.upd.events[1].Type != structs.TaskSetup {
			return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
		}

		if ctx.upd.events[2].Type != structs.TaskDriverFailure {
			return false, fmt.Errorf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskDriverFailure)
		}

		if ctx.upd.events[3].Type != structs.TaskRestarting {
			return false, fmt.Errorf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskRestarting)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_Destroy(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1000s",
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Wait for the task to start
	testWaitForTaskToStart(t, ctx)

	// Begin the tear down
	ctx.tr.Destroy(structs.NewTaskEvent(structs.TaskKilled))

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 5 {
		t.Fatalf("should have 5 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[3].Type != structs.TaskKilling {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskKilling)
	}

	if ctx.upd.events[4].Type != structs.TaskKilled {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskKilled)
	}
}

func TestTaskRunner_Update(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Services[0].Checks[0].Args[0] = "${NOMAD_META_foo}"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "100s",
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Update the task definition
	updateAlloc := ctx.tr.alloc.Copy()

	// Update the restart policy
	newTG := updateAlloc.Job.TaskGroups[0]
	newMode := "foo"
	newTG.RestartPolicy.Mode = newMode

	newTask := newTG.Tasks[0]
	newTask.Driver = "mock_driver"

	// Update meta to make sure service checks are interpolated correctly
	// #2180
	newTask.Meta["foo"] = "UPDATE"

	// Update the kill timeout
	testWaitForTaskToStart(t, ctx)
	oldHandle := ctx.tr.handle.ID()
	newTask.KillTimeout = time.Hour
	ctx.tr.Update(updateAlloc)

	// Wait for ctx.update to take place
	testutil.WaitForResult(func() (bool, error) {
		if ctx.tr.task == newTask {
			return false, fmt.Errorf("We copied the pointer! This would be very bad")
		}
		if ctx.tr.task.Driver != newTask.Driver {
			return false, fmt.Errorf("Task not copied")
		}
		if ctx.tr.restartTracker.policy.Mode != newMode {
			return false, fmt.Errorf("restart policy not ctx.updated")
		}
		if ctx.tr.handle.ID() == oldHandle {
			return false, fmt.Errorf("handle not ctx.updated")
		}
		// Make sure Consul services were interpolated correctly during
		// the update #2180
		consul := ctx.tr.consul.(*mockConsulServiceClient)
		consul.mu.Lock()
		defer consul.mu.Unlock()
		if len(consul.ops) < 2 {
			return false, fmt.Errorf("expected at least 2 consul ops found: %d", len(consul.ops))
		}
		lastOp := consul.ops[len(consul.ops)-1]
		if lastOp.op != "update" {
			return false, fmt.Errorf("expected last consul op to be update not %q", lastOp.op)
		}
		if found := lastOp.task.Services[0].Checks[0].Args[0]; found != "UPDATE" {
			return false, fmt.Errorf("expected consul check to be UPDATE but found: %q", found)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_SaveRestoreState(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "5s",
	}

	// Give it a Vault token
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Wait for the task to be running and then snapshot the state
	testWaitForTaskToStart(t, ctx)

	if err := ctx.tr.SaveState(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read the token from the file system
	tokenPath := filepath.Join(ctx.tr.taskDir.SecretsDir, vaultTokenFile)
	data, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	token := string(data)
	if len(token) == 0 {
		t.Fatalf("Token not written to disk")
	}

	// Create a new task runner
	task2 := &structs.Task{Name: ctx.tr.task.Name, Driver: ctx.tr.task.Driver, Vault: ctx.tr.task.Vault}
	tr2 := NewTaskRunner(ctx.tr.logger, ctx.tr.config, ctx.tr.stateDB, ctx.upd.Update,
		ctx.tr.taskDir, ctx.tr.alloc, task2, ctx.tr.vaultClient, ctx.tr.consul)
	tr2.restartTracker = noRestartsTracker()
	if _, err := tr2.RestoreState(); err != nil {
		t.Fatalf("err: %v", err)
	}
	go tr2.Run()
	defer tr2.Destroy(structs.NewTaskEvent(structs.TaskKilled))

	// Destroy and wait
	select {
	case <-tr2.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that we recovered the token
	if act := tr2.vaultFuture.Get(); act != token {
		t.Fatalf("Vault token not properly recovered")
	}
}

func TestTaskRunner_Download_List(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir("."))))
	defer ts.Close()

	// Create an allocation that has a task with a list of artifacts.
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}
	f1 := "task_runner_test.go"
	f2 := "task_runner.go"
	artifact1 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f1),
	}
	artifact2 := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f2),
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact1, &artifact2}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 5 {
		t.Fatalf("should have 5 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskDownloadingArtifacts)
	}

	if ctx.upd.events[3].Type != structs.TaskStarted {
		t.Fatalf("Forth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskStarted)
	}

	if ctx.upd.events[4].Type != structs.TaskTerminated {
		t.Fatalf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskTerminated)
	}

	// Check that both files exist.
	if _, err := os.Stat(filepath.Join(ctx.tr.taskDir.Dir, f1)); err != nil {
		t.Fatalf("%v not downloaded", f1)
	}
	if _, err := os.Stat(filepath.Join(ctx.tr.taskDir.Dir, f2)); err != nil {
		t.Fatalf("%v not downloaded", f2)
	}
}

func TestTaskRunner_Download_Retries(t *testing.T) {
	t.Parallel()
	// Create an allocation that has a task with bad artifacts.
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}
	artifact := structs.TaskArtifact{
		GetterSource: "http://127.0.0.1:0/foo/bar/baz",
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact}

	// Make the restart policy try one ctx.update
	alloc.Job.TaskGroups[0].RestartPolicy = &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    1 * time.Second,
		Mode:     structs.RestartPolicyModeFail,
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 8 {
		t.Fatalf("should have 8 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskDownloadingArtifacts)
	}

	if ctx.upd.events[3].Type != structs.TaskArtifactDownloadFailed {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskArtifactDownloadFailed)
	}

	if ctx.upd.events[4].Type != structs.TaskRestarting {
		t.Fatalf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskRestarting)
	}

	if ctx.upd.events[5].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Sixth Event was %v; want %v", ctx.upd.events[5].Type, structs.TaskDownloadingArtifacts)
	}

	if ctx.upd.events[6].Type != structs.TaskArtifactDownloadFailed {
		t.Fatalf("Seventh Event was %v; want %v", ctx.upd.events[6].Type, structs.TaskArtifactDownloadFailed)
	}

	if ctx.upd.events[7].Type != structs.TaskNotRestarting {
		t.Fatalf("Eighth Event was %v; want %v", ctx.upd.events[7].Type, structs.TaskNotRestarting)
	}
}

// TestTaskRunner_UnregisterConsul_Retries asserts a task is unregistered from
// Consul when waiting to be retried.
func TestTaskRunner_UnregisterConsul_Retries(t *testing.T) {
	t.Parallel()
	// Create an allocation that has a task with bad artifacts.
	alloc := mock.Alloc()

	// Make the restart policy try one ctx.update
	alloc.Job.TaskGroups[0].RestartPolicy = &structs.RestartPolicy{
		Attempts: 1,
		Interval: 10 * time.Minute,
		Delay:    time.Nanosecond,
		Mode:     structs.RestartPolicyModeFail,
	}

	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "1",
		"run_for":   "1ns",
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	ctx.tr.Run()
	defer ctx.Cleanup()

	// Assert it is properly registered and unregistered
	consul := ctx.tr.consul.(*mockConsulServiceClient)
	if expected := 4; len(consul.ops) != expected {
		t.Errorf("expected %d consul ops but found: %d", expected, len(consul.ops))
	}
	if consul.ops[0].op != "add" {
		t.Errorf("expected first op to be add but found: %q", consul.ops[0].op)
	}
	if consul.ops[1].op != "remove" {
		t.Errorf("expected second op to be remove but found: %q", consul.ops[1].op)
	}
	if consul.ops[2].op != "add" {
		t.Errorf("expected third op to be add but found: %q", consul.ops[2].op)
	}
	if consul.ops[3].op != "remove" {
		t.Errorf("expected fourth/final op to be remove but found: %q", consul.ops[3].op)
	}
}

func TestTaskRunner_Validate_UserEnforcement(t *testing.T) {
	t.Parallel()
	ctx := testTaskRunner(t, false)
	defer ctx.Cleanup()

	// Try to run as root with exec.
	ctx.tr.task.Driver = "exec"
	ctx.tr.task.User = "root"
	if err := ctx.tr.validateTask(); err == nil {
		t.Fatalf("expected error running as root with exec")
	}

	// Try to run a non-blacklisted user with exec.
	ctx.tr.task.Driver = "exec"
	ctx.tr.task.User = "foobar"
	if err := ctx.tr.validateTask(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to run as root with docker.
	ctx.tr.task.Driver = "docker"
	ctx.tr.task.User = "root"
	if err := ctx.tr.validateTask(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskRunner_RestartTask(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "100s",
	}

	ctx := testTaskRunnerFromAlloc(t, true, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Wait for it to start
	go func() {
		testWaitForTaskToStart(t, ctx)
		ctx.tr.Restart("test", "restart")

		// Wait for it to restart then kill
		go func() {
			// Wait for the task to start again
			testutil.WaitForResult(func() (bool, error) {
				if len(ctx.upd.events) != 8 {
					return false, fmt.Errorf("task %q in alloc %q should have 8 ctx.updates: %#v", task.Name, alloc.ID, ctx.upd.events)
				}

				return true, nil
			}, func(err error) {
				t.Fatalf("err: %v", err)
			})
			ctx.tr.Kill("test", "restart", false)
		}()
	}()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 10 {
		t.Fatalf("should have 10 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskRestartSignal {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskRestartSignal)
	}

	if ctx.upd.events[4].Type != structs.TaskKilling {
		t.Fatalf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskKilling)
	}

	if ctx.upd.events[5].Type != structs.TaskKilled {
		t.Fatalf("Sixth Event was %v; want %v", ctx.upd.events[5].Type, structs.TaskKilled)
	}

	if ctx.upd.events[6].Type != structs.TaskRestarting {
		t.Fatalf("Seventh Event was %v; want %v", ctx.upd.events[6].Type, structs.TaskRestarting)
	}

	if ctx.upd.events[7].Type != structs.TaskStarted {
		t.Fatalf("Eighth Event was %v; want %v", ctx.upd.events[8].Type, structs.TaskStarted)
	}
	if ctx.upd.events[8].Type != structs.TaskKilling {
		t.Fatalf("Nineth  Event was %v; want %v", ctx.upd.events[8].Type, structs.TaskKilling)
	}

	if ctx.upd.events[9].Type != structs.TaskKilled {
		t.Fatalf("Tenth Event was %v; want %v", ctx.upd.events[9].Type, structs.TaskKilled)
	}
}

func TestTaskRunner_KillTask(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	go func() {
		testWaitForTaskToStart(t, ctx)
		ctx.tr.Kill("test", "kill", true)
	}()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 5 {
		t.Fatalf("should have 4 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if !ctx.upd.failed {
		t.Fatalf("TaskState should be failed: %+v", ctx.upd)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskKilling {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskKilling)
	}

	if ctx.upd.events[4].Type != structs.TaskKilled {
		t.Fatalf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskKilled)
	}
}

func TestTaskRunner_SignalFailure(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code":    "0",
		"run_for":      "10s",
		"signal_error": "test forcing failure",
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Wait for the task to start
	testWaitForTaskToStart(t, ctx)

	if err := ctx.tr.Signal("test", "test", syscall.SIGINT); err == nil {
		t.Fatalf("Didn't receive error")
	}
}

func TestTaskRunner_BlockForVault(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()

	// Control when we get a Vault token
	token := "1234"
	waitCh := make(chan struct{})
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		<-waitCh
		return map[string]string{task.Name: token}, nil
	}
	ctx.tr.vaultClient.(*vaultclient.MockVaultClient).DeriveTokenFn = handler

	go ctx.tr.Run()

	select {
	case <-ctx.tr.WaitCh():
		t.Fatalf("premature exit")
	case <-time.After(1 * time.Second):
	}

	if len(ctx.upd.events) != 2 {
		t.Fatalf("should have 2 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStatePending {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStatePending)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	// Unblock
	close(waitCh)

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 4 {
		t.Fatalf("should have 4 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskTerminated)
	}

	// Check that the token is on disk
	tokenPath := filepath.Join(ctx.tr.taskDir.SecretsDir, vaultTokenFile)
	data, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if act := string(data); act != token {
		t.Fatalf("Token didn't get written to disk properly, got %q; want %q", act, token)
	}

	// Check the token was revoked
	m := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	testutil.WaitForResult(func() (bool, error) {
		if len(m.StoppedTokens) != 1 {
			return false, fmt.Errorf("Expected a stopped token: %v", m.StoppedTokens)
		}

		if a := m.StoppedTokens[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_DeriveToken_Retry(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()

	// Control when we get a Vault token
	token := "1234"
	count := 0
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		if count > 0 {
			return map[string]string{task.Name: token}, nil
		}

		count++
		return nil, structs.NewRecoverableError(fmt.Errorf("Want a retry"), true)
	}
	ctx.tr.vaultClient.(*vaultclient.MockVaultClient).DeriveTokenFn = handler
	go ctx.tr.Run()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 4 {
		t.Fatalf("should have 4 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskTerminated)
	}

	// Check that the token is on disk
	tokenPath := filepath.Join(ctx.tr.taskDir.SecretsDir, vaultTokenFile)
	data, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if act := string(data); act != token {
		t.Fatalf("Token didn't get written to disk properly, got %q; want %q", act, token)
	}

	// Check the token was revoked
	m := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	testutil.WaitForResult(func() (bool, error) {
		if len(m.StoppedTokens) != 1 {
			return false, fmt.Errorf("Expected a stopped token: %v", m.StoppedTokens)
		}

		if a := m.StoppedTokens[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_DeriveToken_Unrecoverable(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}
	task.Vault = &structs.Vault{
		Policies:   []string{"default"},
		ChangeMode: structs.VaultChangeModeRestart,
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()

	// Error the token derivation
	vc := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	vc.SetDeriveTokenError(alloc.ID, []string{task.Name}, fmt.Errorf("Non recoverable"))
	go ctx.tr.Run()

	// Wait for the task to start
	testutil.WaitForResult(func() (bool, error) {
		if l := len(ctx.upd.events); l != 3 {
			return false, fmt.Errorf("Expect 3 events; got %v", l)
		}

		if ctx.upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
		}

		if ctx.upd.events[1].Type != structs.TaskSetup {
			return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
		}

		if ctx.upd.events[2].Type != structs.TaskKilling {
			return false, fmt.Errorf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskKilling)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_Template_Block(t *testing.T) {
	t.Parallel()
	testRetryRate = 2 * time.Second
	defer func() {
		testRetryRate = 0
	}()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	task.Templates = []*structs.Template{
		{
			EmbeddedTmpl: "{{key \"foo\"}}",
			DestPath:     "local/test",
			ChangeMode:   structs.TemplateChangeModeNoop,
		},
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	select {
	case <-ctx.tr.WaitCh():
		t.Fatalf("premature exit")
	case <-time.After(1 * time.Second):
	}

	if len(ctx.upd.events) != 2 {
		t.Fatalf("should have 2 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStatePending {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStatePending)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	// Unblock
	ctx.tr.UnblockStart("test")

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 4 {
		t.Fatalf("should have 4 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskTerminated)
	}
}

func TestTaskRunner_Template_Artifact(t *testing.T) {
	t.Parallel()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	ts := httptest.NewServer(http.FileServer(http.Dir(filepath.Join(dir, ".."))))
	defer ts.Close()

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	// Create an allocation that has a task that renders a template from an
	// artifact
	f1 := "CHANGELOG.md"
	artifact := structs.TaskArtifact{
		GetterSource: fmt.Sprintf("%s/%s", ts.URL, f1),
	}
	task.Artifacts = []*structs.TaskArtifact{&artifact}
	task.Templates = []*structs.Template{
		{
			SourcePath: "CHANGELOG.md",
			DestPath:   "local/test",
			ChangeMode: structs.TemplateChangeModeNoop,
		},
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()
	go ctx.tr.Run()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 5 {
		t.Fatalf("should have 5 ctx.updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskDownloadingArtifacts {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskDownloadingArtifacts)
	}

	if ctx.upd.events[3].Type != structs.TaskStarted {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskStarted)
	}

	if ctx.upd.events[4].Type != structs.TaskTerminated {
		t.Fatalf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskTerminated)
	}

	// Check that both files exist.
	if _, err := os.Stat(filepath.Join(ctx.tr.taskDir.Dir, f1)); err != nil {
		t.Fatalf("%v not downloaded", f1)
	}
	if _, err := os.Stat(filepath.Join(ctx.tr.taskDir.LocalDir, "test")); err != nil {
		t.Fatalf("template not rendered")
	}
}

func TestTaskRunner_Template_NewVaultToken(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	task.Templates = []*structs.Template{
		{
			EmbeddedTmpl: "{{key \"foo\"}}",
			DestPath:     "local/test",
			ChangeMode:   structs.TemplateChangeModeNoop,
		},
	}
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()
	go ctx.tr.Run()

	// Wait for a Vault token
	var token string
	testutil.WaitForResult(func() (bool, error) {
		if token = ctx.tr.vaultFuture.Get(); token == "" {
			return false, fmt.Errorf("No Vault token")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Error the token renewal
	vc := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	renewalCh, ok := vc.RenewTokens[token]
	if !ok {
		t.Fatalf("no renewal channel")
	}

	originalManager := ctx.tr.templateManager

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	// Wait for a new Vault token
	var token2 string
	testutil.WaitForResult(func() (bool, error) {
		if token2 = ctx.tr.vaultFuture.Get(); token2 == "" || token2 == token {
			return false, fmt.Errorf("No new Vault token")
		}

		if originalManager == ctx.tr.templateManager {
			return false, fmt.Errorf("Template manager not ctx.updated")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check the token was revoked
	m := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	testutil.WaitForResult(func() (bool, error) {
		if len(m.StoppedTokens) != 1 {
			return false, fmt.Errorf("Expected a stopped token: %v", m.StoppedTokens)
		}

		if a := m.StoppedTokens[0]; a != token {
			return false, fmt.Errorf("got stopped token %q; want %q", a, token)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_VaultManager_Restart(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}
	task.Vault = &structs.Vault{
		Policies:   []string{"default"},
		ChangeMode: structs.VaultChangeModeRestart,
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.Cleanup()
	go ctx.tr.Run()

	// Wait for the task to start
	testWaitForTaskToStart(t, ctx)

	// Error the token renewal
	vc := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	renewalCh, ok := vc.RenewTokens[ctx.tr.vaultFuture.Get()]
	if !ok {
		t.Fatalf("no renewal channel")
	}

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	// Ensure a restart
	testutil.WaitForResult(func() (bool, error) {
		if l := len(ctx.upd.events); l != 8 {
			return false, fmt.Errorf("Expect eight events; got %#v", ctx.upd.events)
		}

		if ctx.upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
		}

		if ctx.upd.events[1].Type != structs.TaskSetup {
			return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskStarted)
		}

		if ctx.upd.events[2].Type != structs.TaskStarted {
			return false, fmt.Errorf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
		}

		if ctx.upd.events[3].Type != structs.TaskRestartSignal {
			return false, fmt.Errorf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskRestartSignal)
		}

		if ctx.upd.events[4].Type != structs.TaskKilling {
			return false, fmt.Errorf("Fifth Event was %v; want %v", ctx.upd.events[4].Type, structs.TaskKilling)
		}

		if ctx.upd.events[5].Type != structs.TaskKilled {
			return false, fmt.Errorf("Sixth Event was %v; want %v", ctx.upd.events[5].Type, structs.TaskKilled)
		}

		if ctx.upd.events[6].Type != structs.TaskRestarting {
			return false, fmt.Errorf("Seventh Event was %v; want %v", ctx.upd.events[6].Type, structs.TaskRestarting)
		}

		if ctx.upd.events[7].Type != structs.TaskStarted {
			return false, fmt.Errorf("Eight Event was %v; want %v", ctx.upd.events[7].Type, structs.TaskStarted)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestTaskRunner_VaultManager_Signal(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}
	task.Vault = &structs.Vault{
		Policies:     []string{"default"},
		ChangeMode:   structs.VaultChangeModeSignal,
		ChangeSignal: "SIGUSR1",
	}

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	go ctx.tr.Run()
	defer ctx.Cleanup()

	// Wait for the task to start
	testWaitForTaskToStart(t, ctx)

	// Error the token renewal
	vc := ctx.tr.vaultClient.(*vaultclient.MockVaultClient)
	renewalCh, ok := vc.RenewTokens[ctx.tr.vaultFuture.Get()]
	if !ok {
		t.Fatalf("no renewal channel")
	}

	renewalCh <- fmt.Errorf("Test killing")
	close(renewalCh)

	// Ensure a restart
	testutil.WaitForResult(func() (bool, error) {
		if l := len(ctx.upd.events); l != 4 {
			return false, fmt.Errorf("Expect four events; got %#v", ctx.upd.events)
		}

		if ctx.upd.events[0].Type != structs.TaskReceived {
			return false, fmt.Errorf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
		}

		if ctx.upd.events[1].Type != structs.TaskSetup {
			return false, fmt.Errorf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
		}

		if ctx.upd.events[2].Type != structs.TaskStarted {
			return false, fmt.Errorf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
		}

		if ctx.upd.events[3].Type != structs.TaskSignaling {
			return false, fmt.Errorf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskSignaling)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that the payload is written to disk
func TestTaskRunner_SimpleRun_Dispatch(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}
	fileName := "test"
	task.DispatchPayload = &structs.DispatchPayloadConfig{
		File: fileName,
	}
	alloc.Job.ParameterizedJob = &structs.ParameterizedJobConfig{}

	// Add an encrypted payload
	expected := []byte("hello world")
	compressed := snappy.Encode(nil, expected)
	alloc.Job.Payload = compressed

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()
	defer ctx.tr.Destroy(structs.NewTaskEvent(structs.TaskKilled))
	defer ctx.allocDir.Destroy()
	go ctx.tr.Run()

	select {
	case <-ctx.tr.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	if len(ctx.upd.events) != 4 {
		t.Fatalf("should have 4 updates: %#v", ctx.upd.events)
	}

	if ctx.upd.state != structs.TaskStateDead {
		t.Fatalf("TaskState %v; want %v", ctx.upd.state, structs.TaskStateDead)
	}

	if ctx.upd.events[0].Type != structs.TaskReceived {
		t.Fatalf("First Event was %v; want %v", ctx.upd.events[0].Type, structs.TaskReceived)
	}

	if ctx.upd.events[1].Type != structs.TaskSetup {
		t.Fatalf("Second Event was %v; want %v", ctx.upd.events[1].Type, structs.TaskSetup)
	}

	if ctx.upd.events[2].Type != structs.TaskStarted {
		t.Fatalf("Third Event was %v; want %v", ctx.upd.events[2].Type, structs.TaskStarted)
	}

	if ctx.upd.events[3].Type != structs.TaskTerminated {
		t.Fatalf("Fourth Event was %v; want %v", ctx.upd.events[3].Type, structs.TaskTerminated)
	}

	// Check that the file was written to disk properly
	payloadPath := filepath.Join(ctx.tr.taskDir.LocalDir, fileName)
	data, err := ioutil.ReadFile(payloadPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !reflect.DeepEqual(data, expected) {
		t.Fatalf("Bad; got %v; want %v", string(data), string(expected))
	}
}

// TestTaskRunner_CleanupEmpty ensures TaskRunner works when createdResources
// is empty.
func TestTaskRunner_CleanupEmpty(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.MarkReceived()

	defer ctx.Cleanup()
	ctx.tr.Run()

	// Since we only failed once, createdResources should be empty
	if len(ctx.tr.createdResources.Resources) != 0 {
		t.Fatalf("createdResources should still be empty: %v", ctx.tr.createdResources)
	}
}

func TestTaskRunner_CleanupOK(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	key := "ERR"

	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.config.Options = map[string]string{
		"cleanup_fail_on":  key,
		"cleanup_fail_num": "1",
	}
	ctx.tr.MarkReceived()

	ctx.tr.createdResources.Resources[key] = []string{"x", "y"}
	ctx.tr.createdResources.Resources["foo"] = []string{"z"}

	defer ctx.Cleanup()
	ctx.tr.Run()

	// Since we only failed once, createdResources should be empty
	if len(ctx.tr.createdResources.Resources) > 0 {
		t.Fatalf("expected all created resources to be removed: %#v", ctx.tr.createdResources.Resources)
	}
}

func TestTaskRunner_CleanupFail(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	key := "ERR"
	ctx := testTaskRunnerFromAlloc(t, false, alloc)
	ctx.tr.config.Options = map[string]string{
		"cleanup_fail_on":  key,
		"cleanup_fail_num": "5",
	}
	ctx.tr.MarkReceived()

	ctx.tr.createdResources.Resources[key] = []string{"x"}
	ctx.tr.createdResources.Resources["foo"] = []string{"y", "z"}

	defer ctx.Cleanup()
	ctx.tr.Run()

	// Since we failed > 3 times, the failed key should remain
	expected := map[string][]string{key: {"x"}}
	if !reflect.DeepEqual(expected, ctx.tr.createdResources.Resources) {
		t.Fatalf("expected %#v but found: %#v", expected, ctx.tr.createdResources.Resources)
	}
}

func TestTaskRunner_Pre06ScriptCheck(t *testing.T) {
	t.Parallel()
	run := func(ver, driver, checkType string, exp bool) (string, func(t *testing.T)) {
		name := fmt.Sprintf("%s %s %s returns %t", ver, driver, checkType, exp)
		return name, func(t *testing.T) {
			services := []*structs.Service{
				{
					Checks: []*structs.ServiceCheck{
						{
							Type: checkType,
						},
					},
				},
			}
			if act := pre06ScriptCheck(ver, driver, services); act != exp {
				t.Errorf("expected %t received %t", exp, act)
			}
		}
	}
	t.Run(run("0.5.6", "exec", "script", true))
	t.Run(run("0.5.6", "java", "script", true))
	t.Run(run("0.5.6", "mock_driver", "script", true))
	t.Run(run("0.5.9", "exec", "script", true))
	t.Run(run("0.5.9", "java", "script", true))
	t.Run(run("0.5.9", "mock_driver", "script", true))

	t.Run(run("0.6.0dev", "exec", "script", false))
	t.Run(run("0.6.0dev", "java", "script", false))
	t.Run(run("0.6.0dev", "mock_driver", "script", false))
	t.Run(run("0.6.0", "exec", "script", false))
	t.Run(run("0.6.0", "java", "script", false))
	t.Run(run("0.6.0", "mock_driver", "script", false))
	t.Run(run("1.0.0", "exec", "script", false))
	t.Run(run("1.0.0", "java", "script", false))
	t.Run(run("1.0.0", "mock_driver", "script", false))

	t.Run(run("0.5.6", "rkt", "script", false))
	t.Run(run("0.5.6", "docker", "script", false))
	t.Run(run("0.5.6", "qemu", "script", false))
	t.Run(run("0.5.6", "raw_exec", "script", false))
	t.Run(run("0.5.6", "invalid", "script", false))

	t.Run(run("0.5.6", "exec", "tcp", false))
	t.Run(run("0.5.6", "java", "tcp", false))
	t.Run(run("0.5.6", "mock_driver", "tcp", false))
}
