package allocrunner

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/allochealth"
	cconsul "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// destroy does a blocking destroy on an alloc runner
func destroy(ar *allocRunner) {
	ar.Destroy()
	<-ar.DestroyCh()
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
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0

	// Create two tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "task1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task2"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "1s",
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

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
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 0

	// Create 3 tasks in the task group
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task3 := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task3.Name = "follower2"
	task3.Driver = "mock_driver"
	task3.Config = map[string]interface{}{
		"run_for": "10s",
	}
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2, task3)
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr
	alloc.AllocatedResources.Tasks[task3.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer destroy(ar)
	go ar.Run()

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
	tr := alloc.AllocatedResources.Tasks[alloc.Job.TaskGroups[0].Tasks[0].Name]
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
	alloc.AllocatedResources.Tasks[task.Name] = tr
	alloc.AllocatedResources.Tasks[task2.Name] = tr

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a memory backed statedb
	conf.StateDB = state.NewMemDB()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)

	// Mimic Nomad exiting before the leader stopping is able to stop other tasks.
	ar.tasks["leader"].UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled))
	ar.tasks["follower1"].UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

	// Create a new AllocRunner to test RestoreState and Run
	ar2, err := NewAllocRunner(conf)
	require.NoError(t, err)
	defer destroy(ar2)

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
	case <-ar2.DestroyCh():
		// exited as expected
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for AR to GC")
	}
}

func TestAllocRunner_Update_Semantics(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	updatedAlloc := func(a *structs.Allocation) *structs.Allocation {
		upd := a.CopySkipJob()
		upd.AllocModifyIndex++

		return upd
	}

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	require.NoError(err)

	upd1 := updatedAlloc(alloc)
	ar.Update(upd1)

	// Update was placed into a queue
	require.Len(ar.allocUpdatedCh, 1)

	upd2 := updatedAlloc(alloc)
	ar.Update(upd2)

	// Allocation was _replaced_

	require.Len(ar.allocUpdatedCh, 1)
	queuedAlloc := <-ar.allocUpdatedCh
	require.Equal(upd2, queuedAlloc)

	// Requeueing older alloc is skipped
	ar.Update(upd2)
	ar.Update(upd1)

	queuedAlloc = <-ar.allocUpdatedCh
	require.Equal(upd2, queuedAlloc)

	// Ignore after watch closed

	close(ar.waitCh)

	ar.Update(upd1)

	// Did not queue the update
	require.Len(ar.allocUpdatedCh, 0)
}

// TestAllocRunner_DeploymentHealth_Healthy_Migration asserts that health is
// reported for services that got migrated; not just part of deployments.
func TestAllocRunner_DeploymentHealth_Healthy_Migration(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()

	// Ensure the alloc is *not* part of a deployment
	alloc.DeploymentID = ""

	// Shorten the default migration healthy time
	tg := alloc.Job.TaskGroups[0]
	tg.Migrate = structs.DefaultMigrateStrategy()
	tg.Migrate.MinHealthyTime = 100 * time.Millisecond
	tg.Migrate.HealthCheck = structs.MigrateStrategyHealthStates

	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "30s",
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestAllocRunner_DeploymentHealth_Healthy_NoChecks asserts that the health
// watcher will mark the allocation as healthy based on task states alone.
func TestAllocRunner_DeploymentHealth_Healthy_NoChecks(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()

	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Create a task that takes longer to become healthy
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task.Copy())
	alloc.AllocatedResources.Tasks["task2"] = alloc.AllocatedResources.Tasks["web"].Copy()
	task2 := alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task2"
	task2.Config["start_block_for"] = "500ms"

	// Make the alloc be part of a deployment that uses task states for
	// health checks
	alloc.DeploymentID = uuid.Generate()
	alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)

	start, done := time.Now(), time.Time{}
	go ar.Run()
	defer destroy(ar)

	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status healthy; got unhealthy")
		}

		// Capture the done timestamp
		done = last.DeploymentStatus.Timestamp
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	if d := done.Sub(start); d < 500*time.Millisecond {
		t.Fatalf("didn't wait for second task group. Only took %v", d)
	}
}

// TestAllocRunner_DeploymentHealth_Unhealthy_Checks asserts that the health
// watcher will mark the allocation as unhealthy with failing checks.
func TestAllocRunner_DeploymentHealth_Unhealthy_Checks(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Set a service with check
	task.Services = []*structs.Service{
		{
			Name:      "fakservice",
			PortLabel: "http",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "fakecheck",
					Type:     structs.ServiceCheckScript,
					Command:  "true",
					Interval: 30 * time.Second,
					Timeout:  5 * time.Second,
				},
			},
		},
	}

	// Make the alloc be part of a deployment
	alloc.DeploymentID = uuid.Generate()
	alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond
	alloc.Job.TaskGroups[0].Update.HealthyDeadline = 1 * time.Second

	checkUnhealthy := &api.AgentCheck{
		CheckID: uuid.Generate(),
		Status:  api.HealthWarning,
	}

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Only return the check as healthy after a duration
	consulClient := conf.Consul.(*cconsul.MockConsulServiceClient)
	consulClient.AllocRegistrationsFn = func(allocID string) (*consul.AllocRegistration, error) {
		return &consul.AllocRegistration{
			Tasks: map[string]*consul.TaskRegistration{
				task.Name: {
					Services: map[string]*consul.ServiceRegistration{
						"123": {
							Service: &api.AgentService{Service: "fakeservice"},
							Checks:  []*api.AgentCheck{checkUnhealthy},
						},
					},
				},
			},
		}, nil
	}

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	var lastUpdate *structs.Allocation
	upd := conf.StateUpdater.(*MockStateUpdater)
	testutil.WaitForResult(func() (bool, error) {
		lastUpdate = upd.Last()
		if lastUpdate == nil {
			return false, fmt.Errorf("No updates")
		}
		if !lastUpdate.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *lastUpdate.DeploymentStatus.Healthy {
			// This is fatal
			t.Fatal("want deployment status unhealthy; got healthy")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert that we have an event explaining why we are unhealthy.
	require.Len(t, lastUpdate.TaskStates, 1)
	state := lastUpdate.TaskStates[task.Name]
	require.NotNil(t, state)
	require.NotEmpty(t, state.Events)
	last := state.Events[len(state.Events)-1]
	require.Equal(t, allochealth.AllocHealthEventSource, last.Type)
	require.Contains(t, last.Message, "by deadline")
}

// TestAllocRunner_Destroy asserts that Destroy kills and cleans up a running
// alloc.
func TestAllocRunner_Destroy(t *testing.T) {
	t.Parallel()

	// Ensure task takes some time
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["run_for"] = "10s"

	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	// Use a MemDB to assert alloc state gets cleaned up
	conf.StateDB = state.NewMemDB()

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	go ar.Run()

	// Wait for alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		state := ar.AllocState()

		return state.ClientStatus == structs.AllocClientStatusRunning,
			fmt.Errorf("got client status %v; want running", state.ClientStatus)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Assert state was stored
	ls, ts, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.NotNil(t, ls)
	require.NotNil(t, ts)

	// Now destroy
	ar.Destroy()

	select {
	case <-ar.DestroyCh():
		// Destroyed properly!
	case <-time.After(10 * time.Second):
		require.Fail(t, "timed out waiting for alloc to be destroyed")
	}

	// Assert alloc is dead
	state := ar.AllocState()
	require.Equal(t, structs.AllocClientStatusComplete, state.ClientStatus)

	// Assert the state was cleaned
	ls, ts, err = conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
	require.NoError(t, err)
	require.Nil(t, ls)
	require.Nil(t, ts)

	// Assert the alloc directory was cleaned
	if _, err := os.Stat(ar.allocDir.AllocDir); err == nil {
		require.Fail(t, "alloc dir still exists: %v", ar.allocDir.AllocDir)
	} else if !os.IsNotExist(err) {
		require.Failf(t, "expected NotExist error", "found %v", err)
	}
}
