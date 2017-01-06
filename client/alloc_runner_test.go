package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	"github.com/hashicorp/nomad/client/config"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/client/vaultclient"
)

type MockAllocStateUpdater struct {
	Count  int
	Allocs []*structs.Allocation
}

func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) {
	m.Count += 1
	m.Allocs = append(m.Allocs, alloc)
}

func testAllocRunnerFromAlloc(alloc *structs.Allocation, restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockAllocStateUpdater{}
	if !restarts {
		*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0}
		alloc.Job.Type = structs.JobTypeBatch
	}
	vclient := vaultclient.NewMockVaultClient()
	ar := NewAllocRunner(logger, conf, upd.Update, alloc, vclient)
	return upd, ar
}

func testAllocRunner(restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	return testAllocRunnerFromAlloc(mock.Alloc(), restarts)
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)
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
// retrying fetching an artifact, other tasks in the group should be able
// to proceed.
func TestAllocRunner_RetryArtifact(t *testing.T) {
	ctestutil.ExecCompatible(t)

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
		{GetterSource: "http://127.1.1.111:12315/foo/bar/baz"},
	}

	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, badtask)
	upd, ar := testAllocRunnerFromAlloc(alloc, true)
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
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)

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
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()
	start := time.Now()

	// Begin the tear down
	go func() {
		time.Sleep(1 * time.Second)
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
	ctestutil.ExecCompatible(t)
	_, ar := testAllocRunner(false)

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
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

	upd, ar := testAllocRunnerFromAlloc(alloc, false)
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
		&structs.Allocation{ID: ar.alloc.ID}, ar.vaultClient)
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()

	testutil.WaitForResult(func() (bool, error) {
		if len(ar2.tasks) != 1 {
			return false, fmt.Errorf("Incorrect number of tasks")
		}

		if upd.Count == 0 {
			return false, nil
		}

		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus == structs.AllocClientStatusRunning, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
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
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
	})

	if time.Since(start) > time.Duration(testutil.TestMultiplier()*5)*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_SaveRestoreState_TerminalAlloc(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)
	ar.logger = prefixedTestLogger("ar1: ")

	// Ensure task takes some time

	ar.alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["run_for"] = "10s"
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
		&structs.Allocation{ID: ar.alloc.ID}, ar.vaultClient)
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
		if _, err := os.Stat(ar.allocDir.AllocDir); err != nil {
			return false, fmt.Errorf("alloc dir destroyed: %v", ar.allocDir.AllocDir)
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
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"exit_code": "0",
		"run_for":   "10s",
	}

	logger := testLogger()
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()

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
	ar := NewAllocRunner(logger, conf, upd.Update, alloc, vclient)
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
	if expected := "task runner snapshot includes nil Task"; merr.Errors[0].Error() != expected {
		t.Fatalf("expected %q but got: %q", merr.Errors[0].Error())
	}

	if err := ar.SaveState(); err != nil {
		t.Fatalf("error saving new state: %v", err)
	}
}

func TestAllocRunner_TaskFailed_KillTG(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)

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
		if len(state1.Events) < 3 {
			return false, fmt.Errorf("Unexpected number of events")
		}
		if lastE := state1.Events[len(state1.Events)-3]; lastE.Type != structs.TaskSiblingFailed {
			return false, fmt.Errorf("got last event %v; want %v", lastE.Type, structs.TaskSiblingFailed)
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

func TestAllocRunner_MoveAllocDir(t *testing.T) {
	// Create an alloc runner
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "1s",
	}
	upd, ar := testAllocRunnerFromAlloc(alloc, false)
	go ar.Run()

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

	testutil.WaitForResult(func() (bool, error) {
		if upd1.Count == 0 {
			return false, fmt.Errorf("No updates")
		}
		last := upd1.Allocs[upd1.Count-1]
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
