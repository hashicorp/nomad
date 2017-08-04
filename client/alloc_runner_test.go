package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/vaultclient"
)

type MockAllocStateUpdater struct {
	Allocs []*structs.Allocation
	mu     sync.Mutex
}

// Update fulfills the TaskStateUpdater interface
func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) {
	m.mu.Lock()
	m.Allocs = append(m.Allocs, alloc)
	m.mu.Unlock()
}

// Last returns the total number of updates and the last alloc (or nil)
func (m *MockAllocStateUpdater) Last() (int, *structs.Allocation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Allocs)
	if n == 0 {
		return 0, nil
	}
	return n, m.Allocs[n-1].Copy()
}

func testAllocRunnerFromAlloc(alloc *structs.Allocation, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := config.DefaultConfig()
	conf.Node = mock.Node()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	tmp, _ := ioutil.TempFile("", "state-db")
	db, _ := bolt.Open(tmp.Name(), 0600, nil)
	upd := &MockAllocStateUpdater{}
	if !restarts {
		*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0}
		alloc.Job.Type = structs.JobTypeBatch
	}
	vclient := vaultclient.NewMockVaultClient()
	ar := NewAllocRunner(logger, conf, db, upd.Update, alloc, vclient, newMockConsulServiceClient())
	return upd, ar
}

func testAllocRunner(restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	// Use mock driver
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "500ms"
	return testAllocRunnerFromAlloc(alloc, restarts)
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	t.Parallel()
	upd, ar := testAllocRunner(false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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

// Test that the watcher will mark the allocation as unhealthy.
func TestAllocRunner_DeploymentHealth_Unhealthy_BadStart(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := testAllocRunner(false)

	// Make the task fail
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["start_error"] = "test error"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = structs.GenerateUUID()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status unhealthy; got healthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// Test that the watcher will mark the allocation as unhealthy if it hits its
// deadline.
func TestAllocRunner_DeploymentHealth_Unhealthy_Deadline(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := testAllocRunner(false)

	// Make the task block
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["start_block_for"] = "2s"
	task.Config["run_for"] = "10s"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = structs.GenerateUUID()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.HealthyDeadline = 100 * time.Millisecond

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if *last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status unhealthy; got healthy")
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
	upd, ar := testAllocRunner(false)

	// Make the task run healthy
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"

	// Create a task that takes longer to become healthy
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task.Copy())
	task2 := ar.alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task 2"
	task2.Config["start_block_for"] = "500ms"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = structs.GenerateUUID()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	start := time.Now()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
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
	upd, ar := testAllocRunner(false)

	// Make the task fail
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"

	// Create a task that has no checks
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task.Copy())
	task2 := ar.alloc.Job.TaskGroups[0].Tasks[1]
	task2.Name = "task 2"
	task2.Services = nil

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = structs.GenerateUUID()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_Checks
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	checkHealthy := &api.AgentCheck{
		CheckID: structs.GenerateUUID(),
		Status:  api.HealthPassing,
	}
	checkUnhealthy := &api.AgentCheck{
		CheckID: checkHealthy.CheckID,
		Status:  api.HealthWarning,
	}

	// Only return the check as healthy after a duration
	trigger := time.After(500 * time.Millisecond)
	ar.consulClient.(*mockConsulServiceClient).checksFn = func(a *structs.Allocation) ([]*api.AgentCheck, error) {
		select {
		case <-trigger:
			return []*api.AgentCheck{checkHealthy}, nil
		default:
			return []*api.AgentCheck{checkUnhealthy}, nil
		}
	}

	start := time.Now()
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
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

// Test that the watcher will mark the allocation as healthy.
func TestAllocRunner_DeploymentHealth_Healthy_UpdatedDeployment(t *testing.T) {
	t.Parallel()

	// Ensure the task fails and restarts
	upd, ar := testAllocRunner(false)

	// Make the task run healthy
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "30s"

	// Make the alloc be part of a deployment
	ar.alloc.DeploymentID = structs.GenerateUUID()
	ar.alloc.Job.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	ar.alloc.Job.TaskGroups[0].Update.HealthCheck = structs.UpdateStrategyHealthCheck_TaskStates
	ar.alloc.Job.TaskGroups[0].Update.MaxParallel = 1
	ar.alloc.Job.TaskGroups[0].Update.MinHealthyTime = 100 * time.Millisecond

	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Mimick an update to a new deployment id
	oldCount, last := upd.Last()
	last.DeploymentStatus = nil
	last.DeploymentID = structs.GenerateUUID()
	ar.Update(last)

	testutil.WaitForResult(func() (bool, error) {
		newCount, last := upd.Last()
		if newCount <= oldCount {
			return false, fmt.Errorf("No new updates")
		}
		if last.DeploymentStatus == nil || last.DeploymentStatus.Healthy == nil {
			return false, fmt.Errorf("want deployment status unhealthy; got unset")
		} else if !*last.DeploymentStatus.Healthy {
			return false, fmt.Errorf("want deployment status healthy; got unhealthy")
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
	upd, ar := testAllocRunnerFromAlloc(alloc, true)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		count, last := upd.Last()
		if min := 6; count < min {
			return false, fmt.Errorf("Not enough updates (%d < %d)", count, min)
		}

		// web task should have completed successfully while bad task
		// retries artififact fetching
		webstate := last.TaskStates["web"]
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
	upd, ar := testAllocRunner(false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
		_, last := upd.Last()
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
		_, last := upd.Last()
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
	upd, ar := testAllocRunner(false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"
	go ar.Run()
	start := time.Now()

	// Begin the tear down
	go func() {
		time.Sleep(1 * time.Second)
		ar.Destroy()
	}()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
	_, ar := testAllocRunner(false)

	// Deep copy the alloc to avoid races when updating
	newAlloc := ar.Alloc().Copy()

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config["run_for"] = "10s"
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

	upd, ar := testAllocRunnerFromAlloc(alloc, false)
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
	l2 := prefixedTestLogger("----- ar2:  ")
	ar2 := NewAllocRunner(l2, ar.config, ar.stateDB, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID}, ar.vaultClient,
		ar.consulClient)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()

	testutil.WaitForResult(func() (bool, error) {
		if len(ar2.tasks) != 1 {
			return false, fmt.Errorf("Incorrect number of tasks")
		}

		_, last := upd.Last()
		if last == nil {
			return false, nil
		}

		return last.ClientStatus == structs.AllocClientStatusRunning, nil
	}, func(err error) {
		_, last := upd.Last()
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
		_, last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates)
	})

	if time.Since(start) > time.Duration(testutil.TestMultiplier()*5)*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_SaveRestoreState_TerminalAlloc(t *testing.T) {
	t.Parallel()
	upd, ar := testAllocRunner(false)
	ar.logger = prefixedTestLogger("ar1: ")

	// Ensure task takes some time
	ar.alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["run_for"] = "10s"
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
	ar2 := NewAllocRunner(ar.logger, ar.config, ar.stateDB, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID}, ar.vaultClient, ar.consulClient)
	ar2.logger = prefixedTestLogger("ar2: ")
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
		_, last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar2.logger.Println("[TESTING] destroying second alloc runner")
	ar2.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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

// TestAllocRunner_SaveRestoreState_Upgrade asserts that pre-0.6 exec tasks are
// restarted on upgrade.
func TestAllocRunner_SaveRestoreState_Upgrade(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

	upd, ar := testAllocRunnerFromAlloc(alloc, false)
	// Hack in old version to cause an upgrade on RestoreState
	origConfig := ar.config.Copy()
	ar.config.Version = "0.5.6"
	go ar.Run()
	defer ar.Destroy()

	// Snapshot state
	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
		if last == nil {
			return false, fmt.Errorf("No updates")
		}

		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("task never started: %v", err)
	})

	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new alloc runner
	l2 := prefixedTestLogger("----- ar2:  ")
	ar2 := NewAllocRunner(l2, origConfig, ar.stateDB, upd.Update, &structs.Allocation{ID: ar.alloc.ID}, ar.vaultClient, ar.consulClient)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()
	defer ar2.Destroy() // Just-in-case of failure before Destroy below

	testutil.WaitForResult(func() (bool, error) {
		count, last := upd.Last()
		if min := 3; count < min {
			return false, fmt.Errorf("expected at least %d updates but found %d", min, count)
		}
		for _, ev := range last.TaskStates["web"].Events {
			if strings.HasSuffix(ev.RestartReason, pre06ScriptCheckReason) {
				return true, nil
			}
		}
		return false, fmt.Errorf("no restart with proper reason found")
	}, func(err error) {
		count, last := upd.Last()
		t.Fatalf("err: %v\nAllocs: %d\nweb state: % #v", err, count, pretty.Formatter(last.TaskStates["web"]))
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
		_, last := upd.Last()
		t.Fatalf("err: %v %#v %#v", err, last, last.TaskStates)
	})

	if time.Since(start) > time.Duration(testutil.TestMultiplier()*5)*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

// Ensure pre-#2132 state files containing the Context struct are properly
// migrated to the new format.
//
// Old Context State:
//
//  "Context": {
//    "AllocDir": {
//      "AllocDir": "/path/to/allocs/2a54fcff-fc44-8d4f-e025-53c48e9cbbbb",
//      "SharedDir": "/path/to/allocs/2a54fcff-fc44-8d4f-e025-53c48e9cbbbb/alloc",
//      "TaskDirs": {
//        "echo1": "/path/to/allocs/2a54fcff-fc44-8d4f-e025-53c48e9cbbbb/echo1"
//      }
//    },
//    "AllocID": "2a54fcff-fc44-8d4f-e025-53c48e9cbbbb"
//  }
func TestAllocRunner_RestoreOldState(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

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

	if err := os.MkdirAll(filepath.Join(conf.StateDir, "alloc", alloc.ID), 0777); err != nil {
		t.Fatalf("error creating state dir: %v", err)
	}
	statePath := filepath.Join(conf.StateDir, "alloc", alloc.ID, "state.json")
	w, err := os.Create(statePath)
	if err != nil {
		t.Fatalf("error creating state file: %v", err)
	}
	tmplctx := &struct {
		AllocID  string
		AllocDir string
	}{alloc.ID, conf.AllocDir}
	err = template.Must(template.New("test_state").Parse(`{
  "Version": "0.5.1",
  "Alloc": {
    "ID": "{{ .AllocID }}",
    "Name": "example",
    "JobID": "example",
    "Job": {
      "ID": "example",
      "Name": "example",
      "Type": "batch",
      "TaskGroups": [
        {
          "Name": "example",
          "Tasks": [
            {
              "Name": "example",
              "Driver": "mock",
              "Config": {
                "exit_code": "0",
		"run_for": "10s"
              }
            }
          ]
        }
      ]
    },
    "TaskGroup": "example",
    "DesiredStatus": "run",
    "ClientStatus": "running",
    "TaskStates": {
      "example": {
        "State": "running",
        "Failed": false,
        "Events": []
      }
    }
  },
  "Context": {
    "AllocDir": {
      "AllocDir": "{{ .AllocDir }}/{{ .AllocID }}",
      "SharedDir": "{{ .AllocDir }}/{{ .AllocID }}/alloc",
      "TaskDirs": {
        "example": "{{ .AllocDir }}/{{ .AllocID }}/example"
      }
    },
    "AllocID": "{{ .AllocID }}"
  }
}`)).Execute(w, tmplctx)
	if err != nil {
		t.Fatalf("error writing state file: %v", err)
	}
	w.Close()

	upd := &MockAllocStateUpdater{}
	*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0}
	alloc.Job.Type = structs.JobTypeBatch
	vclient := vaultclient.NewMockVaultClient()
	cclient := newMockConsulServiceClient()
	ar := NewAllocRunner(logger, conf, db, upd.Update, alloc, vclient, cclient)
	defer ar.Destroy()

	// RestoreState should fail on the task state since we only test the
	// alloc state restoring.
	err = ar.RestoreState()
	if err == nil {
		t.Fatal("expected error restoring Task state")
	}
	merr, ok := err.(*multierror.Error)
	if !ok {
		t.Fatalf("expected RestoreState to return a multierror but found: %T -> %v", err, err)
	}
	if len(merr.Errors) != 1 {
		t.Fatalf("expected exactly 1 error from RestoreState but found: %d: %v", len(merr.Errors), err)
	}
	if expected := "failed to get task bucket"; !strings.Contains(merr.Errors[0].Error(), expected) {
		t.Fatalf("expected %q but got: %q", expected, merr.Errors[0].Error())
	}

	if err := ar.SaveState(); err != nil {
		t.Fatalf("error saving new state: %v", err)
	}
}

func TestAllocRunner_TaskFailed_KillTG(t *testing.T) {
	t.Parallel()
	upd, ar := testAllocRunner(false)

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
	ar.alloc.TaskResources[task2.Name] = task2.Resources
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
	upd, ar := testAllocRunner(false)

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
	ar.alloc.TaskResources[task2.Name] = task2.Resources
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
	upd, ar := testAllocRunner(false)

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
	ar.alloc.TaskResources[task2.Name] = task2.Resources
	defer ar.Destroy()

	go ar.Run()

	// Wait for tasks to start
	oldCount, last := upd.Last()
	testutil.WaitForResult(func() (bool, error) {
		oldCount, last = upd.Last()
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

	// Stop alloc
	update := ar.Alloc()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(update)

	// Wait for tasks to stop
	testutil.WaitForResult(func() (bool, error) {
		newCount, last := upd.Last()
		if newCount == oldCount {
			return false, fmt.Errorf("no new updates (count: %d)", newCount)
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
		count, last := upd.Last()
		t.Logf("Updates: %d", count)
		for name, state := range last.TaskStates {
			t.Logf("%s: %s", name, state.State)
		}
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_MoveAllocDir(t *testing.T) {
	t.Parallel()
	// Create an alloc runner
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}
	upd, ar := testAllocRunnerFromAlloc(alloc, false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd.Last()
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
	alloc1 := mock.Alloc()
	task = alloc1.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}
	upd1, ar1 := testAllocRunnerFromAlloc(alloc1, false)
	ar1.SetPreviousAllocDir(ar.allocDir)
	go ar1.Run()
	defer ar1.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		_, last := upd1.Last()
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

	// Ensure that data from ar1 was moved to ar
	taskDir = ar1.allocDir.TaskDirs[task.Name]
	taskLocalFile = filepath.Join(taskDir.LocalDir, "local_file")
	if fileInfo, _ := os.Stat(taskLocalFile); fileInfo == nil {
		t.Fatalf("file %v not found", taskLocalFile)
	}

	dataFile = filepath.Join(ar1.allocDir.SharedDir, "data", "data_file")
	if fileInfo, _ := os.Stat(dataFile); fileInfo == nil {
		t.Fatalf("file %v not found", dataFile)
	}
}
