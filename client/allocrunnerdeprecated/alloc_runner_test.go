// +build deprecated

package allocrunner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/nomad/client/allocrunnerdeprecated/taskrunner"
	consulApi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/state"
	"github.com/stretchr/testify/require"
)

// allocationBucketExists checks if the allocation bucket was created.
func allocationBucketExists(tx *bolt.Tx, allocID string) bool {
	bucket, err := state.GetAllocationBucket(tx, allocID)
	return err == nil && bucket != nil
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that FinisheAt is set when the alloc is in a terminal state
func TestAllocRunner_FinishedAtSet(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, ar := TestAllocRunner(t, false)
	ar.allocClientStatus = structs.AllocClientStatusFailed
	alloc := ar.Alloc()
	taskFinishedAt := make(map[string]time.Time)
	require.NotEmpty(alloc.TaskStates)
	for name, s := range alloc.TaskStates {
		require.False(s.FinishedAt.IsZero())
		taskFinishedAt[name] = s.FinishedAt
	}

	// Verify that calling again should not mutate finishedAt
	alloc2 := ar.Alloc()
	for name, s := range alloc2.TaskStates {
		require.Equal(taskFinishedAt[name], s.FinishedAt)
	}

}

// Test that FinisheAt is set when the alloc is in a terminal state
func TestAllocRunner_FinishedAtSet_TaskEvents(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, ar := TestAllocRunner(t, false)
	ar.taskStates[ar.alloc.Job.TaskGroups[0].Tasks[0].Name] = &structs.TaskState{State: structs.TaskStateDead, Failed: true}

	alloc := ar.Alloc()
	taskFinishedAt := make(map[string]time.Time)
	require.NotEmpty(alloc.TaskStates)
	for name, s := range alloc.TaskStates {
		require.False(s.FinishedAt.IsZero())
		taskFinishedAt[name] = s.FinishedAt
	}

	// Verify that calling again should not mutate finishedAt
	alloc2 := ar.Alloc()
	for name, s := range alloc2.TaskStates {
		require.Equal(taskFinishedAt[name], s.FinishedAt)
	}

}

// Test that the watcher will mark the allocation as unhealthy.
func TestAllocRunner_DeploymentHealth_Unhealthy_BadStart(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task fail
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["start_error"] = "test error"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status unhealthy; got healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Assert that we have an event explaining why we are unhealthy.
	assert.Len(ar.taskStates, 1)
	state := ar.taskStates[task.Name]
	assert.NotNil(state)
	assert.NotEmpty(state.Events)
	last := state.Events[len(state.Events)-1]
	assert.Equal(allocHealthEventSource, last.Type)
	assert.Contains(last.Message, "failed task")
}

// Test that the watcher will mark the allocation as unhealthy if it hits its
// deadline.
func TestAllocRunner_DeploymentHealth_Unhealthy_Deadline(t *testing.T) {
	t.Parallel()

	// Don't restart but force service job type
	upd, ar := TestAllocRunner(t, false)
	ar.alloc.Job.Type = structs.JobTypeService

	// Make the task block
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"start_block_for": "4s",
		"run_for":         "10s",
	}

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.HealthyDeadline = 100 * time.Millisecond

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Assert alloc is unhealthy
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status unhealthy; got healthy")
		}

		// Assert there is a task event explaining why we are unhealthy.
		state, ok := last.TaskStates[task.Name]
		if !ok {
			return false, fmt.Errorf("missing state for task %s", task.Name)
		}
		n := len(state.Events)
		if n == 0 {
			return false, fmt.Errorf("no task events")
		}
		lastEvent := state.Events[n-1]
		if lastEvent.Type != allocHealthEventSource {
			return false, fmt.Errorf("expected %q; found %q", allocHealthEventSource, lastEvent.Type)
		}
		if !strings.Contains(lastEvent.Message, "not running by deadline") {
			return false, fmt.Errorf(`expected "not running by deadline" but found: %s`, lastEvent.Message)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that the watcher will mark the allocation as healthy.
func TestAllocRunner_DeploymentHealth_Healthy_NoChecks(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task run healthy
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Create a task that takes longer to become healthy
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task.Copy())
	task2 := ar.alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task 2"
	task2.Config["start_block_for"] = "500ms"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	start := time.Now()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
	if d := time.Now().Sub(start); d < 500*time.Millisecond {
		t.Fatalf("didn't wait for second task group. Only took %v", d)
	}
}

// Test that the watcher will mark the allocation as healthy with checks
func TestAllocRunner_DeploymentHealth_Healthy_Checks(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task fail
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Create a task that has no checks
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task.Copy())
	task2 := ar.alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task 2"
	task2.Services = nil

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	checkHealthy := &api.AgentCheck{
		CheckID: uuid.Generate(),
		Status:  api.HealthPassing,
	}
	checkUnhealthy := &api.AgentCheck{
		CheckID: checkHealthy.CheckID,
		Status:  api.HealthWarning,
	}

	// Only return the check as healthy after a duration
	trigger := time.After(500 * time.Millisecond)
	ar.consulClient.(*consulApi.MockConsulServiceClient).AllocRegistrationsFn = func(allocID string) (*consul.AllocRegistration, error) {
		select {
		case <-trigger:
			return &consul.AllocRegistration{
				Tasks: map[string]*consul.TaskRegistration{
					task.Name: {
						Services: map[string]*consul.ServiceRegistration{
							"123": {
								Service: &api.AgentService{Service: "foo"},
								Checks:  []*api.AgentCheck{checkHealthy},
							},
						},
					},
				},
			}, nil
		default:
			return &consul.AllocRegistration{
				Tasks: map[string]*consul.TaskRegistration{
					task.Name: {
						Services: map[string]*consul.ServiceRegistration{
							"123": {
								Service: &api.AgentService{Service: "foo"},
								Checks:  []*api.AgentCheck{checkUnhealthy},
							},
						},
					},
				},
			}, nil
		}
	}

	start := time.Now()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	if d := time.Now().Sub(start); d < 500*time.Millisecond {
		t.Fatalf("didn't wait for second task group. Only took %v", d)
	}
}

// Test that the watcher will mark the allocation as unhealthy with failing
// checks
func TestAllocRunner_DeploymentHealth_Unhealthy_Checks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task fail
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond
	ar.alloc.Job.TaskGroups[0].Update.HealthyDeadline = 1 * time.Second

	checkUnhealthy := &api.AgentCheck{
		CheckID: uuid.Generate(),
		Status:  api.HealthWarning,
	}

	// Only return the check as healthy after a duration
	ar.consulClient.(*consulApi.MockConsulServiceClient).AllocRegistrationsFn = func(allocID string) (*consul.AllocRegistration, error) {
		return &consul.AllocRegistration{
			Tasks: map[string]*consul.TaskRegistration{
				task.Name: {
					Services: map[string]*consul.ServiceRegistration{
						"123": {
							Service: &api.AgentService{Service: "foo"},
							Checks:  []*api.AgentCheck{checkUnhealthy},
						},
					},
				},
			},
		}, nil
	}

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status unhealthy; got healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Assert that we have an event explaining why we are unhealthy.
	assert.Len(ar.taskStates, 1)
	state := ar.taskStates[task.Name]
	assert.NotNil(state)
	assert.NotEmpty(state.Events)
	last := state.Events[len(state.Events)-1]
	assert.Equal(allocHealthEventSource, last.Type)
	assert.Contains(last.Message, "Services not healthy by deadline")
}

// Test that the watcher will mark the allocation as healthy.
func TestAllocRunner_DeploymentHealth_Healthy_UpdatedDeployment(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task run healthy
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "30s",
	}

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = uuid.Generate()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Mimick an update to a new deployment id
	last := upd.Last()
	last.DeploymentStatus = nil
	last.DeploymentID = uuid.Generate()
	ar.Update(last)

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that health is reported for services that got migrated; not just part
// of deployments.
func TestAllocRunner_DeploymentHealth_Healthy_Migration(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := TestAllocRunner(t, true)

	// Make the task run healthy
	tg := ar.alloc.Job.TaskGroups[0]
	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "30s",
	}

	// Shorten the default migration healthy time
	tg.Migrate = structs.DefaultMigrateStrategy()
	tg.Migrate.MinHealthyTime = 100 * time.Millisecond
	tg.Migrate.HealthCheck = structs.MigrateStrategyHealthStates

	// Ensure the alloc is *not* part of a deployment
	ar.alloc.DeploymentID = ""

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if !last.DeploymentStatus.HasHealth() {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that health is *not* reported for batch jobs
func TestAllocRunner_DeploymentHealth_BatchDisabled(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	alloc := mock.BatchAlloc()
	tg := alloc.Job.TaskGroups[0]

	// This should not be possile as validation should prevent batch jobs
	// from having a migration stanza!
	tg.Migrate = structs.DefaultMigrateStrategy()
	tg.Migrate.MinHealthyTime = 1 * time.Millisecond
	tg.Migrate.HealthyDeadline = 2 * time.Millisecond
	tg.Migrate.HealthCheck = structs.MigrateStrategyHealthStates

	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "5s",
	}
	upd, ar := TestAllocRunnerFromAlloc(t, alloc, false)

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus != nil {
			return false, fmt.Errorf("unexpected deployment health set: %v", last.DeploymentStatus.Healthy)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRuner_RetryArtifact ensures that if one task in a task group is
// retrying fetching an artifact, other tasks in the group should be able
// to proceed.
func TestAllocRunner_RetryArtifact(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].RestartPolicy.Mode = structs.RestartPolicyModeFail
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 1
	alloc.Job.TaskGroups[0].RestartPolicy.Delay = time.Duration(4*testutil.TestMultiplier()) * time.Second

	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "1s",
	}

	// Create a new task with a bad artifact
	badtask := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	badtask.Name = "bad"
	badtask.Artifacts = []*structs.TaskArtifact{
		{GetterSource: "http://127.0.0.1:0/foo/bar/baz"},
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, badtask)
	upd, ar := TestAllocRunnerFromAlloc(t, alloc, true)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// web task should have completed successfully while bad task
		// retries artifact fetching
		webstate, ok := last.TaskStates["web"]
		if !ok {
			return false, fmt.Errorf("no task state for web")
		}
		if webstate.State != structs.TaskStateDead {
			return false, fmt.Errorf("expected web to be dead but found %q", last.TaskStates["web"].State)
		}
		if !webstate.Successful() {
			return false, fmt.Errorf("expected web to have exited successfully")
		}

		// bad task should have failed
		badstate := last.TaskStates["bad"]
		if badstate.State != structs.TaskStateDead {
			return false, fmt.Errorf("expected bad to be dead but found %q", badstate.State)
		}
		if !badstate.Failed {
			return false, fmt.Errorf("expected bad to have failed: %#v", badstate.Events)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_TerminalUpdate_Destroy(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Update the alloc to be terminal which should cause the alloc runner to
	// stop the tasks and wait for a destroy.
	update := ar.alloc.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the allocation state still exists
		if err := ar.stateDB.View(func(tx *bolt.Tx) error {
			if !allocationBucketExists(tx, ar.Alloc().ID) {
				return fmt.Errorf("no bucket for alloc")
			}

			return nil
		}); err != nil {
			return false, fmt.Errorf("state destroyed")
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.allocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.allocDir.AllocDir)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if err := ar.stateDB.View(func(tx *bolt.Tx) error {
			if allocationBucketExists(tx, ar.Alloc().ID) {
				return fmt.Errorf("bucket for alloc exists")
			}

			return nil
		}); err != nil {
			return false, fmt.Errorf("state not destroyed")
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.allocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.allocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_Destroy(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	go ar.Run()
	start := time.Now()

	// Begin the tear down
	go func() {
		time.Sleep(1 * time.Second)
		ar.Destroy()
	}()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if err := ar.stateDB.View(func(tx *bolt.Tx) error {
			if allocationBucketExists(tx, ar.Alloc().ID) {
				return fmt.Errorf("bucket for alloc exists")
			}

			return nil
		}); err != nil {
			return false, fmt.Errorf("state not destroyed: %v", err)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.allocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.allocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("took too long to terminate: %s", elapsed)
	}
}

func TestAllocRunner_Update(t *testing.T) {
	t.Parallel()
	_, ar := TestAllocRunner(t, false)

	// Deep copy the alloc to avoid races when updating
	newAlloc := ar.Alloc().Copy()

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	go ar.Run()
	defer ar.Destroy()

	// Update the alloc definition
	newAlloc.Name = "FOO"
	newAlloc.AllocModifyIndex++
	ar.Update(newAlloc)

	// Check the alloc runner stores the update allocation.
	testutil.WaitForResult(func() (bool, error) {
		return ar.Alloc().Name == "FOO", nil
	}, func(err error) {
		t.Fatalf("err: %v %#v", err, ar.Alloc())
	})
}

func TestAllocRunner_SaveRestoreState(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

	upd, ar := TestAllocRunnerFromAlloc(t, alloc, false)
	go ar.Run()
	defer ar.Destroy()

	// Snapshot state
	testutil.WaitForResult(func() (bool, error) {
		ar.taskLock.RLock()
		defer ar.taskLock.RUnlock()
		return len(ar.tasks) == 1, nil
	}, func(err error) {
		t.Fatalf("task never started: %v", err)
	})

	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new alloc runner
	l2 := testlog.WithPrefix(t, "----- ar2:  ")
	alloc2 := &structs.Allocation{ID: ar.alloc.ID}
	prevAlloc := NewAllocWatcher(alloc2, ar, nil, ar.config, l2, "")
	ar2 := NewAllocRunner(l2, ar.config, ar.stateDB, upd.Update,
		alloc2, ar.vaultClient, ar.consulClient, prevAlloc)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()

	testutil.WaitForResult(func() (bool, error) {
		if len(ar2.tasks) != 1 {
			return false, fmt.Errorf("Incorrect number of tasks")
		}

		last := upd.Last()
		if last == nil {
			return false, nil
		}

		return last.ClientStatus == structs.AllocClientStatusRunning, nil
	}, func(err error) {
		last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates["web"])
	})

	// Destroy and wait
	ar2.Destroy()
	start := time.Now()

	testutil.WaitForResult(func() (bool, error) {
		alloc := ar2.Alloc()
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("Bad client status; got %v; want %v", alloc.ClientStatus, structs.AllocClientStatusComplete)
		}
		return true, nil
	}, func(err error) {
		last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates)
	})

	if time.Since(start) > time.Duration(testutil.TestMultiplier()*5)*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_SaveRestoreState_TerminalAlloc(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)
	ar.logger = testlog.WithPrefix(t, "ar1:  ")

	// Ensure task takes some time
	ar.alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Update the alloc to be terminal which should cause the alloc runner to
	// stop the tasks and wait for a destroy.
	update := ar.alloc.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	testutil.WaitForResult(func() (bool, error) {
		return ar.Alloc().DesiredStatus == structs.AllocDesiredStatusStop, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure ar1 doesn't recreate the state file
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()

	// Create a new alloc runner
	l2 := testlog.WithPrefix(t, "ar2:  ")
	alloc2 := &structs.Allocation{ID: ar.alloc.ID}
	prevAlloc := NewAllocWatcher(alloc2, ar, nil, ar.config, l2, "")
	ar2 := NewAllocRunner(l2, ar.config, ar.stateDB, upd.Update,
		alloc2, ar.vaultClient, ar.consulClient, prevAlloc)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	ar2.logger.Println("[TESTING] running second alloc runner")
	go ar2.Run()
	defer ar2.Destroy() // Just-in-case of failure before Destroy below

	testutil.WaitForResult(func() (bool, error) {
		// Check the state still exists
		if err := ar.stateDB.View(func(tx *bolt.Tx) error {
			if !allocationBucketExists(tx, ar2.Alloc().ID) {
				return fmt.Errorf("no bucket for alloc")
			}

			return nil
		}); err != nil {
			return false, fmt.Errorf("state destroyed")
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.allocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.allocDir.AllocDir)
		}

		return true, nil
	}, func(err error) {
		last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar2.logger.Println("[TESTING] destroying second alloc runner")
	ar2.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		// Check the status has changed.
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if err := ar.stateDB.View(func(tx *bolt.Tx) error {
			if allocationBucketExists(tx, ar2.Alloc().ID) {
				return fmt.Errorf("bucket for alloc exists")
			}

			return nil
		}); err != nil {
			return false, fmt.Errorf("state not destroyed")
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.allocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.allocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_TaskFailed_KillTG(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)

	// Create two tasks in the task group
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task 2"
	task2.Driver = "mock_driver"
	task2.Config = map[string]interface{}{
		"start_error": "fail task please",
	}
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task2)
	ar.alloc.AllocatedResources.Tasks[task2.Name] = ar.alloc.AllocatedResources.Tasks[task.Name].Copy()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusFailed)
		}

		// Task One should be killed
		state1 := last.TaskStates[task.Name]
		if state1.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state1.State, structs.TaskStateDead)
		}
		if len(state1.Events) < 2 {
			// At least have a received and destroyed
			return false, fmt.Errorf("Unexpected number of events")
		}

		found := false
		for _, e := range state1.Events {
			if e.Type != structs.TaskSiblingFailed {
				found = true
			}
		}

		if !found {
			return false, fmt.Errorf("Did not find event %v", structs.TaskSiblingFailed)
		}

		// Task Two should be failed
		state2 := last.TaskStates[task2.Name]
		if state2.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state2.State, structs.TaskStateDead)
		}
		if !state2.Failed {
			return false, fmt.Errorf("task2 should have failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_TaskLeader_KillTG(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)

	// Create two tasks in the task group
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task 2"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.Config = map[string]interface{}{
		"run_for": "1s",
	}
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task2)
	ar.alloc.AllocatedResources.Tasks[task2.Name] = ar.alloc.AllocatedResources.Tasks[task.Name].Copy()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Task One should be killed
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

// TestAllocRunner_TaskLeader_StopTG asserts that when stopping a task group
// with a leader the leader is stopped before other tasks.
func TestAllocRunner_TaskLeader_StopTG(t *testing.T) {
	t.Parallel()
	upd, ar := TestAllocRunner(t, false)

	// Create 3 tasks in the task group
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Millisecond
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.KillTimeout = 10 * time.Millisecond
	task2.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task3 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task3.Name = "follower2"
	task3.Driver = "mock_driver"
	task3.KillTimeout = 10 * time.Millisecond
	task3.Config = map[string]interface{}{
		"run_for": "10s",
	}
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task2, task3)
	ar.alloc.AllocatedResources.Tasks[task.Name] = ar.alloc.AllocatedResources.Tasks["web"].Copy()
	ar.alloc.AllocatedResources.Tasks[task2.Name] = ar.alloc.AllocatedResources.Tasks[task.Name].Copy()
	ar.alloc.AllocatedResources.Tasks[task3.Name] = ar.alloc.AllocatedResources.Tasks[task.Name].Copy()
	defer ar.Destroy()

	go ar.Run()

	// Wait for tasks to start
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
	upd.mu.Lock()
	upd.Allocs = upd.Allocs[:0]
	upd.mu.Unlock()

	// Stop alloc
	update := ar.Alloc()
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
	t.Skip("Skipping because the functionality being tested doesn't exist")
	t.Parallel()
	_, ar := TestAllocRunner(t, false)
	defer ar.Destroy()

	// Create a leader and follower task in the task group
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "follower1"
	task.Driver = "mock_driver"
	task.KillTimeout = 10 * time.Second
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}

	task2 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "leader"
	task2.Driver = "mock_driver"
	task2.Leader = true
	task2.KillTimeout = 10 * time.Millisecond
	task2.Config = map[string]interface{}{
		"run_for": "0s",
	}

	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task2)
	ar.alloc.AllocatedResources.Tasks[task.Name] = ar.alloc.AllocatedResources.Tasks["web"].Copy()
	ar.alloc.AllocatedResources.Tasks[task2.Name] = ar.alloc.AllocatedResources.Tasks[task.Name].Copy()

	// Mimic Nomad exiting before the leader stopping is able to stop other tasks.
	ar.tasks = map[string]*taskrunner.TaskRunner{
		"leader": taskrunner.NewTaskRunner(ar.logger, ar.config, ar.stateDB, ar.setTaskState,
			ar.allocDir.NewTaskDir(task2.Name), ar.Alloc(), task2.Copy(),
			ar.vaultClient, ar.consulClient),
		"follower1": taskrunner.NewTaskRunner(ar.logger, ar.config, ar.stateDB, ar.setTaskState,
			ar.allocDir.NewTaskDir(task.Name), ar.Alloc(), task.Copy(),
			ar.vaultClient, ar.consulClient),
	}
	ar.taskStates = map[string]*structs.TaskState{
		"leader":    {State: structs.TaskStateDead},
		"follower1": {State: structs.TaskStateRunning},
	}
	if err := ar.SaveState(); err != nil {
		t.Fatalf("error saving state: %v", err)
	}

	// Create a new AllocRunner to test RestoreState and Run
	upd2 := &MockAllocStateUpdater{}
	ar2 := NewAllocRunner(ar.logger, ar.config, ar.stateDB, upd2.Update, ar.alloc,
		ar.vaultClient, ar.consulClient, ar.prevAlloc)
	defer ar2.Destroy()

	if err := ar2.RestoreState(); err != nil {
		t.Fatalf("error restoring state: %v", err)
	}
	go ar2.Run()

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

// TestAllocRunner_MoveAllocDir asserts that a file written to an alloc's
// local/ dir will be moved to a replacement alloc's local/ dir if sticky
// volumes is on.
func TestAllocRunner_MoveAllocDir(t *testing.T) {
	t.Parallel()
	// Create an alloc runner
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}
	upd, ar := TestAllocRunnerFromAlloc(t, alloc, false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Write some data in data dir and task dir of the alloc
	dataFile := filepath.Join(ar.allocDir.SharedDir, "data", "data_file")
	ioutil.WriteFile(dataFile, []byte("hello world"), os.ModePerm)
	taskDir := ar.allocDir.TaskDirs[task.Name]
	taskLocalFile := filepath.Join(taskDir.LocalDir, "local_file")
	ioutil.WriteFile(taskLocalFile, []byte("good bye world"), os.ModePerm)

	// Create another alloc runner
	alloc2 := mock.Alloc()
	alloc2.PreviousAllocation = ar.allocID
	alloc2.Job.TaskGroups[0].EphemeralDisk.Sticky = true
	task = alloc2.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}
	upd2, ar2 := TestAllocRunnerFromAlloc(t, alloc2, false)

	// Set prevAlloc like Client does
	ar2.prevAlloc = NewAllocWatcher(alloc2, ar, nil, ar2.config, ar2.logger, "")

	go ar2.Run()
	defer ar2.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		last := upd2.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Ensure that data from ar was moved to ar2
	taskDir = ar2.allocDir.TaskDirs[task.Name]
	taskLocalFile = filepath.Join(taskDir.LocalDir, "local_file")
	if fileInfo, _ := os.Stat(taskLocalFile); fileInfo == nil {
		t.Fatalf("file %v not found", taskLocalFile)
	}

	dataFile = filepath.Join(ar2.allocDir.SharedDir, "data", "data_file")
	if fileInfo, _ := os.Stat(dataFile); fileInfo == nil {
		t.Fatalf("file %v not found", dataFile)
	}
}
