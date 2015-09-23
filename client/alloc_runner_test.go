package client

import (
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

func testAllocRunner() (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = "/tmp"
	upd := &MockAllocStateUpdater{}
	alloc := mock.Alloc()
	ar := NewAllocRunner(logger, conf, upd.Update, alloc)
	return upd, ar
}

func TestAllocRunner_SimpleRun(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner()
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
	upd, ar := testAllocRunner()

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = "10"
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
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.taskStatus)
	})

	if time.Since(start) > time.Second {
		t.Fatalf("took too long to terminate")
	}
}

func TestAllocRunner_Update(t *testing.T) {
	ctestutil.ExecCompatible(t)
	upd, ar := testAllocRunner()

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = "10"
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
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.taskStatus)
	})

	if time.Since(start) > time.Second {
		t.Fatalf("took too long to terminate")
	}
}

/*
TODO: This test is disabled til a follow-up api changes the restore state interface.
The driver/executor interface will be changed from Open to Cleanup, in which
clean-up tears down previous allocs.

func TestAllocRunner_SaveRestoreState(t *testing.T) {
	upd, ar := testAllocRunner()

	// Ensure task takes some time
	task := ar.alloc.Job.TaskGroups[0].Tasks[0]
	task.Config["command"] = "/bin/sleep"
	task.Config["args"] = "10"
	go ar.Run()
	defer ar.Destroy()

	// Snapshot state
	time.Sleep(200 * time.Millisecond)
	err := ar.SaveState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new alloc runner
	ar2 := NewAllocRunner(ar.logger, ar.config, upd.Update,
		&structs.Allocation{ID: ar.alloc.ID})
	err = ar2.RestoreState()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	go ar2.Run()
	defer ar2.Destroy()

	// Destroy and wait
	ar2.Destroy()
	start := time.Now()

	testutil.WaitForResult(func() (bool, error) {
		if upd.Count == 0 {
			return false, nil
		}
		last := upd.Allocs[upd.Count-1]
		return last.ClientStatus == structs.AllocClientStatusDead, nil
	}, func(err error) {
		t.Fatalf("err: %v %#v %#v", err, upd.Allocs[0], ar.taskStatus)
	})

	if time.Since(start) > time.Second {
		t.Fatalf("took too long to terminate")
	}
}
*/
