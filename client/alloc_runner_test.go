package client

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

type MockAllocStateUpdater struct {
	Count  int
	Allocs []*structs.Allocation
	Err    error
}

func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) error {
	m.Count += 1
	m.Allocs = append(m.Allocs, alloc)
	return m.Err
}

func testAllocRunner(restarts bool) (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	upd := &MockAllocStateUpdater{}
	alloc := mock.Alloc()
	consulClient, _ := NewConsulService(&consulServiceConfig{logger, "127.0.0.1:8500", "", "", false, false, &structs.Node{}})
	if !restarts {
		*alloc.Job.LookupTaskGroup(alloc.TaskGroup).RestartPolicy = structs.RestartPolicy{Attempts: 0, RestartOnSuccess: false}
	}

	ar := NewAllocRunner(logger, conf, upd.Update, alloc, consulClient)
	return upd, ar
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)
	go ar.Run()
	defer ar.Destroy()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}
		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus == structs.AllocClientStatusDead, nil
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
		time.Sleep(100 * time.Millisecond)
		ar.Destroy()
	}()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}
		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus == structs.AllocClientStatusDead, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
	})

	if time.Since(start) > 15*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_Update(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()
	defer ar.Destroy()
	start := time.Now()

	// Update the alloc definition
	newAlloc := new(structs.Allocation)
	*newAlloc = *ar.alloc
	newAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	ar.Update(newAlloc)

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}
		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus == structs.AllocClientStatusDead, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.alloc.TaskStates)
	})

	if time.Since(start) > 15*time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_SaveRestoreState(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner(false)

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = []string{"10"}
	go ar.Run()
	defer ar.Destroy()

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
	consulClient, err := NewConsulService(&consulServiceConfig{ar.logger, "127.0.0.1:8500", "", "", false, false, &structs.Node{}})
	ar2 := NewAllocRunner(ar.logger, ar.config, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID}, consulClient)
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
