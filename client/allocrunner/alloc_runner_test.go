package allocrunner

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/config"
	consulapi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/shared/catalog"
	"github.com/hashicorp/nomad/plugins/shared/singleton"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// MockStateUpdater implements the AllocStateHandler interface and records
// alloc updates.
type MockStateUpdater struct {
	Updates []*structs.Allocation
	mu      sync.Mutex
}

// AllocStateUpdated implements the AllocStateHandler interface and records an
// alloc update.
func (m *MockStateUpdater) AllocStateUpdated(alloc *structs.Allocation) {
	m.mu.Lock()
	m.Updates = append(m.Updates, alloc)
	m.mu.Unlock()
}

// Last returns a copy of the last alloc (or nil) update. Safe for concurrent
// access with updates.
func (m *MockStateUpdater) Last() *structs.Allocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Updates)
	if n == 0 {
		return nil
	}
	return m.Updates[n-1].Copy()
}

// Reset resets the recorded alloc updates.
func (m *MockStateUpdater) Reset() {
	m.mu.Lock()
	m.Updates = nil
	m.mu.Unlock()
}

// testAllocRunnerConfig returns a new allocrunner.Config with mocks and noop
// versions of dependencies along with a cleanup func.
func testAllocRunnerConfig(t *testing.T, alloc *structs.Allocation) (*Config, func()) {
	pluginLoader := catalog.TestPluginLoader(t)
	clientConf, cleanup := config.TestClientConfig(t)
	conf := &Config{
		// Copy the alloc in case the caller edits and reuses it
		Alloc:                 alloc.Copy(),
		Logger:                clientConf.Logger,
		ClientConfig:          clientConf,
		StateDB:               state.NoopDB{},
		Consul:                consulapi.NewMockConsulServiceClient(t, clientConf.Logger),
		Vault:                 vaultclient.NewMockVaultClient(),
		StateUpdater:          &MockStateUpdater{},
		PrevAllocWatcher:      allocwatcher.NoopPrevAlloc{},
		PluginSingletonLoader: singleton.NewSingletonLoader(clientConf.Logger, pluginLoader),
	}
	return conf, cleanup
}

// TestAllocRunner_AllocState_Initialized asserts that getting TaskStates via
// AllocState() are initialized even before the AllocRunner has run.
func TestAllocRunner_AllocState_Initialized(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)

	allocState := ar.AllocState()

	require.NotNil(t, allocState)
	require.NotNil(t, allocState.TaskStates[conf.Alloc.Job.TaskGroups[0].Tasks[0].Name])
}

// TestAllocRunner_TaskLeader_KillTG asserts that when a leader task dies the
// entire task group is killed.
func TestAllocRunner_TaskLeader_KillTG(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0

	// Create two tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "task1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": 10 * time.Second,
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task2"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": 1 * time.Second,
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.TaskResources[task2.Name] = task2.Resources

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer ar.Destroy()
	ar.Run()

	// Wait for all tasks to be killed
	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Task1 should be killed because Task2 exited
		state1 := last.TaskStates[task.Name]
		if state1.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state1.State, structs.TaskStateDead)
		}
		if state1.FinishedAt.IsZero() || state1.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}
		if len(state1.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		found := false
		for _, e := range state1.Events {
			if e.Type != structs.TaskLeaderDead {
				found = true
			}
		}

		if !found {
			return false, fmt.Errorf("Did not find event %v", structs.TaskLeaderDead)
		}

		// Task Two should be dead
		state2 := last.TaskStates[task2.Name]
		if state2.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state2.State, structs.TaskStateDead)
		}
		if state2.FinishedAt.IsZero() || state2.StartedAt.IsZero() {
			return false, fmt.Errorf("expected to have a start and finish time")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRunner_TaskLeader_StopTG asserts that when stopping an alloc with a
// leader the leader is stopped before other tasks.
func TestAllocRunner_TaskLeader_StopTG(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0

	// Create 3 tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": 10 * time.Second,
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": 10 * time.Second,
	}

	task3 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task3.Name = "follower2"
	task3.Driver = "mock_driver"
	task3.Config = map[string]interface{}{
		"run_for": 10 * time.Second,
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2, task3)
	alloc.TaskResources[task2.Name] = task2.Resources

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer ar.Destroy()
	ar.Run()

	// Wait for tasks to start
	upd := conf.StateUpdater.(*MockStateUpdater)
	last := upd.Last()
	testutil.WaitForResult(func() (bool, error) {
		last = upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if n := len(last.TaskStates); n != 3 {
			return false, fmt.Errorf("Not enough task states (want: 3; found %d)", n)
		}
		for name, state := range last.TaskStates {
			if state.State != structs.TaskStateRunning {
				return false, fmt.Errorf("Task %q is not running yet (it's %q)", name, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Reset updates
	upd.Reset()

	// Stop alloc
	update := alloc.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	// Wait for tasks to stop
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.TaskStates["leader"].FinishedAt.UnixNano() >= last.TaskStates["follower1"].FinishedAt.UnixNano() {
			return false, fmt.Errorf("expected leader to finish before follower1: %s >= %s",
				last.TaskStates["leader"].FinishedAt, last.TaskStates["follower1"].FinishedAt)
		}
		if last.TaskStates["leader"].FinishedAt.UnixNano() >= last.TaskStates["follower2"].FinishedAt.UnixNano() {
			return false, fmt.Errorf("expected leader to finish before follower2: %s >= %s",
				last.TaskStates["leader"].FinishedAt, last.TaskStates["follower2"].FinishedAt)
		}
		return true, nil
	}, func(err error) {
		last := upd.Last()
		for name, state := range last.TaskStates {
			t.Logf("%s: %s", name, state.State)
		}
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRunner_TaskLeader_StopRestoredTG asserts that when stopping a
// restored task group with a leader that failed before restoring the leader is
// not stopped as it does not exist.
// See https://github.com/hashicorp/nomad/issues/3420#issuecomment-341666932
func TestAllocRunner_TaskLeader_StopRestoredTG(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0

	// Create a leader and follower task in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Second
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.KillTimeout = 10 * time.Millisecond
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.TaskResources[task2.Name] = task2.Resources

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a memory backed statedb
	conf.StateDB = state.NewMemDB()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer ar.Destroy()

	// Mimic Nomad exiting before the leader stopping is able to stop other tasks.
	ar.tasks["leader"].UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled))
	ar.tasks["follower1"].UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

	// Create a new AllocRunner to test RestoreState and Run
	ar2, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer ar2.Destroy()

	if err := ar2.Restore(); err != nil {
		t.Fatalf("error restoring state: %v", err)
	}
	ar2.Run()

	// Wait for tasks to be stopped because leader is dead
	testutil.WaitForResult(func() (bool, error) {
		alloc := ar2.Alloc()
		for task, state := range alloc.TaskStates {
			if state.State != structs.TaskStateDead {
				return false, fmt.Errorf("Task %q should be dead: %v", task, state.State)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Make sure it GCs properly
	ar2.Destroy()

	select {
	case <-ar2.WaitCh():
		// exited as expected
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for AR to GC")
	}
}

/*

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func testAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation) *allocRunner {
	cconf := clientconfig.DefaultConfig()
	config := &Config{
		ClientConfig: cconf,
		Logger:       testlog.HCLogger(t).With("unit_test", t.Name()),
		Alloc:        alloc,
	}

	ar := NewAllocRunner(config)
	return ar
}

func testAllocRunner(t *testing.T) *allocRunner {
	return testAllocRunnerFromAlloc(t, mock.Alloc())
}

// preRun is a test RunnerHook that captures whether Prerun was called on it
type preRun struct{ run bool }

func (p *preRun) Name() string { return "pre" }
func (p *preRun) Prerun() error {
	p.run = true
	return nil
}

// postRun is a test RunnerHook that captures whether Postrun was called on it
type postRun struct{ run bool }

func (p *postRun) Name() string { return "post" }
func (p *postRun) Postrun() error {
	p.run = true
	return nil
}

// Tests that prerun only runs pre run hooks.
func TestAllocRunner_Prerun_Basic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ar := testAllocRunner(t)

	// Overwrite the hooks with test hooks
	pre := &preRun{}
	post := &postRun{}
	ar.runnerHooks = []interfaces.RunnerHook{pre, post}

	// Run the hooks
	require.NoError(ar.prerun())

	// Assert only the pre is run
	require.True(pre.run)
	require.False(post.run)
}

// Tests that postrun only runs post run hooks.
func TestAllocRunner_Postrun_Basic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ar := testAllocRunner(t)

	// Overwrite the hooks with test hooks
	pre := &preRun{}
	post := &postRun{}
	ar.runnerHooks = []interfaces.RunnerHook{pre, post}

	// Run the hooks
	require.NoError(ar.postrun())

	// Assert only the pre is run
	require.True(post.run)
	require.False(pre.run)
}
*/
