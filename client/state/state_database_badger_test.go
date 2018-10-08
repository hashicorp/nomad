package state_test

import (
	"os"
	"testing"

	trstate "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestBadgerGetAllocsEmpty(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()
	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}
	allocs, errMap, err := db.GetAllAllocations()
	if err != nil {
		t.Fatal("GetAllAllocations failed", err)
	}
	if len(errMap) > 0 {
		t.Fatal("GetAllAllocations failed", errMap)
	}
	if len(allocs) > 0 {
		t.Fatal("GetAllAllocations failed - more than one allocation")
	}
}

func TestBadgerPutGetAllocs(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()
	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	err = db.PutAllocation(&structs.Allocation{
		ID: "Alloc01",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	allocs, errMap, err := db.GetAllAllocations()
	if err != nil {
		t.Fatal("GetAllAllocations failed", err)
	}
	if len(errMap) > 0 {
		t.Fatal("GetAllAllocations failed", errMap)
	}
	if len(allocs) != 1 {
		t.Fatal("GetAllAllocations failed - exactly one allocation expected")
	}
	if allocs[0].ID != "Alloc01" {
		t.Fatal("GetAllAllocations failed - unexpected value - ID != Alloc01")
	}
}

func TestBadgerPutDelAllocs(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()
	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	err = db.PutAllocation(&structs.Allocation{
		ID: "Alloc02",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	err = db.DeleteAllocationBucket("Alloc02")
	if err != nil {
		t.Fatal("DeleteAllocationBucket failed", err)
	}

	allocs, errMap, err := db.GetAllAllocations()
	if err != nil {
		t.Fatal("GetAllAllocations failed", err)
	}
	if len(errMap) > 0 {
		t.Fatal("GetAllAllocations failed", errMap)
	}
	if len(allocs) == 1 {
		t.Fatal("GetAllAllocations failed - no allocation expected")
	}
}

func TestBadgerGetTaskRunnerStateEmpty(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	ls, ts, err := db.GetTaskRunnerState("Alloc03", "task03")
	if err == nil || err.Error() != "failed to get task \"task03\" bucket: Allocations bucket doesn't exist and transaction is not writable" {
		t.Fatal("GetTaskRunnerState is expected to return an error", err)
	}
	if ls != nil {
		t.Fatal("GetTaskRunnerState returned non nil local state")
	}
	if ts != nil {
		t.Fatal("GetTaskRunnerState returned non nil task state")
	}
}

func TestBadgerTaskStateEmpty(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	err = db.PutAllocation(&structs.Allocation{
		ID: "Alloc03",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	err = db.PutTaskRunnerLocalState("Alloc03", "task03", &trstate.LocalState{})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	ls, ts, err := db.GetTaskRunnerState("Alloc03", "task03")
	if err == nil || err.Error() != "failed to read task state: no data at key task_state" {
		t.Fatal("GetTaskRunnerState failed", err)
	}
	if ls == nil {
		t.Fatal("GetTaskRunnerState returned nil local state")
	}
	if ts != nil {
		t.Fatal("GetTaskRunnerState returned non nil task state")
	}
}

func TestBadgerTaskLocalStateEmpty(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	err = db.PutAllocation(&structs.Allocation{
		ID: "Alloc03",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	err = db.PutTaskState("Alloc03", "task03", &structs.TaskState{
		//TODO State: "test State",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	ls, ts, err := db.GetTaskRunnerState("Alloc03", "task03")
	if err == nil || err.Error() != "failed to read local task runner state: no data at key local_state" {
		t.Fatal("GetTaskRunnerState failed", err)
	}
	if ls != nil {
		t.Fatal("GetTaskRunnerState returned non nil local state")
	}
	if ts == nil {
		t.Fatal("GetTaskRunnerState returned nil task state")
	}
}

func TestBadgerTaskStateLocal(t *testing.T) {
	dir := os.TempDir()
	db, err := state.NewBadgerStateDB(dir)
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	if err != nil {
		t.Fatal("NewBadgerStateDB failed", err)
	}

	err = db.PutAllocation(&structs.Allocation{
		ID: "Alloc03",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	err = db.PutTaskRunnerLocalState("Alloc03", "task03", &trstate.LocalState{})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	err = db.PutTaskState("Alloc03", "task03", &structs.TaskState{
		//TODO State: "test State",
	})
	if err != nil {
		t.Fatal("PutAllocation failed", err)
	}

	ls, ts, err := db.GetTaskRunnerState("Alloc03", "task03")
	if err != nil {
		t.Fatal("GetTaskRunnerState failed", err)
	}
	if ls == nil {
		t.Fatal("GetTaskRunnerState returned nil local state")
	}
	if ts == nil {
		t.Fatal("GetTaskRunnerState returned nil task state")
	}

	err = db.DeleteTaskBucket("Alloc03", "task03")
	if err != nil {
		t.Fatal("DeleteTaskBucket failed", err)
	}

	allocs, errMap, err := db.GetAllAllocations()
	if err != nil {
		t.Fatal("GetAllAllocations failed", err)
	}
	if len(errMap) > 0 {
		t.Fatal("GetAllAllocations failed", errMap)
	}
	if len(allocs) != 1 {
		t.Fatal("GetAllAllocations failed - exactly one allocation expected")
	}
}
