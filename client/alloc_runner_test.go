package client

import (
	"bufio"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/secretdir"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

type MockAllocStateUpdater struct {
	Count  int
	Allocs []*structs.Allocation
}

func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) {
	m.Count += 1
	m.Allocs = append(m.Allocs, alloc)
}

func testAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockAllocStateUpdater{}
	if !restarts {
		*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0}
		alloc.Job.Type = structs.JobTypeBatch
	}
	ar := NewAllocRunner(logger, conf, upd.Update, alloc, secretdir.NewTestSecretDir(t))
	return upd, ar
}

func testAllocRunner(t *testing.T, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	return testAllocRunnerFromAlloc(t, mock.Alloc(), restarts)
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

// TestAllocRuner_RetryArtifact ensures that if one task in a task group is
// retrying fetching an artifact, other tasks in the the group should be able
// to proceed.
func TestAllocRunner_RetryArtifact(t *testing.T) {
	ctestutil.ExecCompatible(t)

	alloc := mock.Alloc()
	alloc.Job.Type = structs.JobTypeBatch
	alloc.Job.TaskGroups[0].RestartPolicy.Attempts = 1
	alloc.Job.TaskGroups[0].RestartPolicy.Delay = time.Duration(4*testutil.TestMultiplier()) * time.Second

	// Create a new task with a bad artifact
	badtask := alloc.Job.TaskGroups[0].Tasks[0].Copy()
	badtask.Name = "bad"
	badtask.Artifacts = []*structs.TaskArtifact{
		{GetterSource: "http://127.1.1.111:12315/foo/bar/baz"},
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, badtask)
	upd, ar := testAllocRunnerFromAlloc(t, alloc, true)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count < 6 {
			return false, fmt.Errorf("Not enough updates")
		}
		last := upd.Allocs[upd.Count-1]

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
			return false, fmt.Errorf("expected bad to be dead but found %q", last.TaskStates["web"].State)
		}
		if !badstate.Failed() {
			return false, fmt.Errorf("expected bad to have failed")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_TerminalUpdate_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd.Allocs[upd.Count-1]
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
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state still exists
		if _, err := os.Stat(ar.stateFilePath()); err != nil {
			return false, fmt.Errorf("state file destroyed: %v", err)
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.ctx.AllocDir.AllocDir)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if _, err := os.Stat(ar.stateFilePath()); err == nil {
			return false, fmt.Errorf("state file still exists: %v", ar.stateFilePath())
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.ctx.AllocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_DiskExceeded_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"60"}
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusRunning)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Create a 20mb file in the alloc directory, which should cause the
	// allocation to terminate in a failed state.
	name := ar.ctx.AllocDir.AllocDir + "/20mb.bin"
	f, err := os.Create(name)
	if err != nil {
		t.Fatal("unable to create file: %v", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal("unable to close file: %v", err)
		}
		os.Remove(name)
	}()

	// write 20 megabytes (1280 * 16384 bytes) of zeros to the file
	w := bufio.NewWriter(f)
	buf := make([]byte, 16384)
	for i := 0; i < 1280; i++ {
		if _, err := w.Write(buf); err != nil {
			t.Fatal("unable to write to file: %v", err)
		}
	}

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusFailed)
		}

		// Check the state still exists
		if _, err := os.Stat(ar.stateFilePath()); err != nil {
			return false, fmt.Errorf("state file destroyed: %v", err)
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.ctx.AllocDir.AllocDir)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusFailed)
		}

		// Check the state was cleaned
		if _, err := os.Stat(ar.stateFilePath()); err == nil {
			return false, fmt.Errorf("state file still exists: %v", ar.stateFilePath())
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.ctx.AllocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
func TestAllocRunner_Destroy(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()
	start := time.Now()

	// Begin the tear down
	go func() {
		time.Sleep(100 * time.Millisecond)
		ar.Destroy()
	}()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if _, err := os.Stat(ar.stateFilePath()); err == nil {
			return false, fmt.Errorf("state file still exists: %v", ar.stateFilePath())
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.ctx.AllocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	if time.Since(start) > 15*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_Update(t *testing.T) {
	ctestutil.ExecCompatible(t)
	_, ar := testAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()
	defer ar.Destroy()

	// Update the alloc definition
	newAlloc := new(structs.Allocation)
	*newAlloc = *ar.alloc
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
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()

	// Snapshot state
	testutil.WaitForResult(func() (bool, error) {
		return len(ar.tasks) == 1, nil
	}, func(err error) {
		t.Fatalf("task never started: %v", err)
	})

	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new alloc runner
	ar2 := NewAllocRunner(ar.logger, ar.config, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID}, ar.secretDir)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()

	// Destroy and wait
	ar2.Destroy()
	start := time.Now()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}
		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus != structs.AllocClientStatusPending, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
	})

	if time.Since(start) > time.Duration(testutil.TestMultiplier()*15)*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_SaveRestoreState_TerminalAlloc(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)
	ar.logger = prefixedTestLogger("ar1: ")

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"1000"}
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd.Allocs[upd.Count-1]
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
		return ar.alloc.DesiredStatus == structs.AllocDesiredStatusStop, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure both alloc runners don't destroy
	ar.destroy = true

	// Create a new alloc runner
	ar2 := NewAllocRunner(ar.logger, ar.config, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID}, ar.secretDir)
	ar2.logger = prefixedTestLogger("ar2: ")
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()
	ar2.logger.Println("[TESTING] starting second alloc runner")

	testutil.WaitForResult(func() (bool, error) {
		// Check the state still exists
		if _, err := os.Stat(ar.stateFilePath()); err != nil {
			return false, fmt.Errorf("state file destroyed: %v", err)
		}

		// Check the alloc directory still exists
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.ctx.AllocDir.AllocDir)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
	})

	// Send the destroy signal and ensure the AllocRunner cleans up.
	ar2.logger.Println("[TESTING] destroying second alloc runner")
	ar2.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}

		// Check the status has changed.
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("got client status %v; want %v", last.ClientStatus, structs.AllocClientStatusComplete)
		}

		// Check the state was cleaned
		if _, err := os.Stat(ar.stateFilePath()); err == nil {
			return false, fmt.Errorf("state file still exists: %v", ar.stateFilePath())
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		// Check the alloc directory was cleaned
		if _, err := os.Stat(ar.ctx.AllocDir.AllocDir); err == nil {
			return false, fmt.Errorf("alloc dir still exists: %v", ar.ctx.AllocDir.AllocDir)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("stat err: %v", err)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocRunner_TaskFailed_KillTG(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(t, false)

	// Create two tasks in the task group
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"1000"}

	task2 := ar.alloc.Job.TaskGroups[0].Tasks[0].Copy()
	task2.Name = "task 2"
	task2.Config = map[string]interface{}{"command": "invalidBinaryToFail"}
	ar.alloc.Job.TaskGroups[0].Tasks = append(ar.alloc.Job.TaskGroups[0].Tasks, task2)
	ar.alloc.TaskResources[task2.Name] = task2.Resources
	//t.Logf("%#v", ar.alloc.Job.TaskGroups[0])
	go ar.Run()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd.Allocs[upd.Count-1]
		if last.ClientStatus != structs.AllocClientStatusFailed {
			return false, fmt.Errorf("got status %v; want %v", last.ClientStatus, structs.AllocClientStatusFailed)
		}

		// Task One should be killed
		state1 := last.TaskStates[task.Name]
		if state1.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state1.State, structs.TaskStateDead)
		}
		if lastE := state1.Events[len(state1.Events)-1]; lastE.Type != structs.TaskSiblingFailed {
			return false, fmt.Errorf("got last event %v; want %v", lastE.Type, structs.TaskSiblingFailed)
		}

		// Task Two should be failed
		state2 := last.TaskStates[task2.Name]
		if state2.State != structs.TaskStateDead {
			return false, fmt.Errorf("got state %v; want %v", state2.State, structs.TaskStateDead)
		}
		if !state2.Failed() {
			return false, fmt.Errorf("task2 should have failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
