// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// assert each implementation satisfies StateDB interface
var (
	_ StateDB = (*BoltStateDB)(nil)
	_ StateDB = (*MemDB)(nil)
	_ StateDB = (*NoopDB)(nil)
	_ StateDB = (*ErrDB)(nil)
)

func setupBoltStateDB(t *testing.T) *BoltStateDB {
	dir := t.TempDir()

	db, err := NewBoltStateDB(testlog.HCLogger(t), dir)
	if err != nil {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			t.Logf("error removing boltdb dir: %v", rmErr)
		}
		t.Fatalf("error creating boltdb: %v", err)
	}

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("error closing boltdb: %v", closeErr)
		}
	})

	return db.(*BoltStateDB)
}

func testDB(t *testing.T, f func(*testing.T, StateDB)) {
	dbs := []StateDB{
		setupBoltStateDB(t),
		NewMemDB(testlog.HCLogger(t)),
	}

	for _, db := range dbs {
		t.Run(db.Name(), func(t *testing.T) {
			f(t, db)
		})
	}
}

// TestStateDB_Allocations asserts the behavior of GetAllAllocations, PutAllocation, and
// DeleteAllocationBucket for all operational StateDB implementations.
func TestStateDB_Allocations(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// Empty database should return empty non-nil results
		allocs, errs, err := db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(allocs)
		require.Empty(allocs)
		require.NotNil(errs)
		require.Empty(errs)

		// Put allocations
		alloc1 := mock.Alloc()
		alloc2 := mock.BatchAlloc()

		require.NoError(db.PutAllocation(alloc1))
		require.NoError(db.PutAllocation(alloc2))

		// Retrieve them
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(allocs)
		require.Len(allocs, 2)
		for _, a := range allocs {
			switch a.ID {
			case alloc1.ID:
				if !reflect.DeepEqual(a, alloc1) {
					pretty.Ldiff(t, a, alloc1)
					t.Fatalf("alloc %q unequal", a.ID)
				}
			case alloc2.ID:
				if !reflect.DeepEqual(a, alloc2) {
					pretty.Ldiff(t, a, alloc2)
					t.Fatalf("alloc %q unequal", a.ID)
				}
			default:
				t.Fatalf("unexpected alloc id %q", a.ID)
			}
		}
		require.NotNil(errs)
		require.Empty(errs)

		// Add another
		alloc3 := mock.SystemAlloc()
		require.NoError(db.PutAllocation(alloc3))
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(allocs)
		require.Len(allocs, 3)
		require.Contains(allocs, alloc1)
		require.Contains(allocs, alloc2)
		require.Contains(allocs, alloc3)
		require.NotNil(errs)
		require.Empty(errs)

		// Deleting a nonexistent alloc is a noop
		require.NoError(db.DeleteAllocationBucket("asdf"))
		allocs, _, err = db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(allocs)
		require.Len(allocs, 3)

		// Delete alloc1
		require.NoError(db.DeleteAllocationBucket(alloc1.ID))
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(allocs)
		require.Len(allocs, 2)
		require.Contains(allocs, alloc2)
		require.Contains(allocs, alloc3)
		require.NotNil(errs)
		require.Empty(errs)
	})
}

// Integer division, rounded up.
func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

// TestStateDB_Batch asserts the behavior of PutAllocation, PutNetworkStatus and
// DeleteAllocationBucket in batch mode, for all operational StateDB implementations.
func TestStateDB_Batch(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// For BoltDB, get initial tx_id
		var getTxID func() int
		var prevTxID int
		var batchDelay time.Duration
		var batchSize int
		if boltStateDB, ok := db.(*BoltStateDB); ok {
			boltdb := boltStateDB.DB().BoltDB()
			getTxID = func() int {
				tx, err := boltdb.Begin(true)
				require.NoError(err)
				defer tx.Rollback()
				return tx.ID()
			}
			prevTxID = getTxID()
			batchDelay = boltdb.MaxBatchDelay
			batchSize = boltdb.MaxBatchSize
		}

		// Write 1000 allocations and network statuses in batch mode
		startTime := time.Now()
		const numAllocs = 1000
		var allocs []*structs.Allocation
		for i := 0; i < numAllocs; i++ {
			allocs = append(allocs, mock.Alloc())
		}
		var wg sync.WaitGroup
		for _, alloc := range allocs {
			wg.Add(1)
			go func(alloc *structs.Allocation) {
				require.NoError(db.PutNetworkStatus(alloc.ID, mock.AllocNetworkStatus(), WithBatchMode()))
				require.NoError(db.PutAllocation(alloc, WithBatchMode()))
				wg.Done()
			}(alloc)
		}
		wg.Wait()

		// Check BoltDB actually combined PutAllocation calls into much fewer transactions.
		// The actual number of transactions depends on how fast the goroutines are spawned,
		// with every batchDelay (10ms by default) period saved in a separate transaction,
		// plus each transaction is limited to batchSize writes (1000 by default).
		// See boltdb MaxBatchDelay and MaxBatchSize parameters for more details.
		if getTxID != nil {
			numTransactions := getTxID() - prevTxID
			writeTime := time.Now().Sub(startTime)
			expectedNumTransactions := ceilDiv(2*numAllocs, batchSize) + ceilDiv(int(writeTime), int(batchDelay))
			require.LessOrEqual(numTransactions, expectedNumTransactions)
			prevTxID = getTxID()
		}

		// Retrieve allocs and make sure they are the same (order can differ)
		readAllocs, errs, err := db.GetAllAllocations()
		require.NoError(err)
		require.NotNil(readAllocs)
		require.Len(readAllocs, len(allocs))
		require.NotNil(errs)
		require.Empty(errs)

		readAllocsById := make(map[string]*structs.Allocation)
		for _, readAlloc := range readAllocs {
			readAllocsById[readAlloc.ID] = readAlloc
		}
		for _, alloc := range allocs {
			readAlloc, ok := readAllocsById[alloc.ID]
			if !ok {
				t.Fatalf("no alloc with ID=%q", alloc.ID)
			}
			if !reflect.DeepEqual(readAlloc, alloc) {
				pretty.Ldiff(t, readAlloc, alloc)
				t.Fatalf("alloc %q unequal", alloc.ID)
			}
		}

		// Delete all allocs in batch mode
		startTime = time.Now()
		for _, alloc := range allocs {
			wg.Add(1)
			go func(alloc *structs.Allocation) {
				require.NoError(db.DeleteAllocationBucket(alloc.ID, WithBatchMode()))
				wg.Done()
			}(alloc)
		}
		wg.Wait()

		// Check BoltDB combined DeleteAllocationBucket calls into much fewer transactions.
		if getTxID != nil {
			numTransactions := getTxID() - prevTxID
			writeTime := time.Now().Sub(startTime)
			expectedNumTransactions := ceilDiv(numAllocs, batchSize) + ceilDiv(int(writeTime), int(batchDelay))
			require.LessOrEqual(numTransactions, expectedNumTransactions)
			prevTxID = getTxID()
		}

		// Check all allocs were deleted.
		readAllocs, errs, err = db.GetAllAllocations()
		require.NoError(err)
		require.Empty(readAllocs)
		require.Empty(errs)
	})
}

// TestStateDB_TaskState asserts the behavior of task state related StateDB
// methods.
func TestStateDB_TaskState(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// Getting nonexistent state should return nils
		ls, ts, err := db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(err)
		require.Nil(ls)
		require.Nil(ts)

		// Putting TaskState without first putting the allocation should work
		state := structs.NewTaskState()
		state.Failed = true // set a non-default value
		require.NoError(db.PutTaskState("allocid", "taskname", state))

		// Getting should return the available state
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(err)
		require.Nil(ls)
		require.Equal(state, ts)

		// Deleting a nonexistent task should not error
		require.NoError(db.DeleteTaskBucket("adsf", "asdf"))
		require.NoError(db.DeleteTaskBucket("asllocid", "asdf"))

		// Data should be untouched
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(err)
		require.Nil(ls)
		require.Equal(state, ts)

		// Deleting the task should remove the state
		require.NoError(db.DeleteTaskBucket("allocid", "taskname"))
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(err)
		require.Nil(ls)
		require.Nil(ts)

		// Putting LocalState should work just like TaskState
		origLocalState := trstate.NewLocalState()
		require.NoError(db.PutTaskRunnerLocalState("allocid", "taskname", origLocalState))
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(err)
		require.Equal(origLocalState, ls)
		require.Nil(ts)
	})
}

// TestStateDB_DeviceManager asserts the behavior of device manager state related StateDB
// methods.
func TestStateDB_DeviceManager(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// Getting nonexistent state should return nils
		ps, err := db.GetDevicePluginState()
		require.NoError(err)
		require.Nil(ps)

		// Putting PluginState should work
		state := &dmstate.PluginState{}
		require.NoError(db.PutDevicePluginState(state))

		// Getting should return the available state
		ps, err = db.GetDevicePluginState()
		require.NoError(err)
		require.NotNil(ps)
		require.Equal(state, ps)
	})
}

// TestStateDB_DriverManager asserts the behavior of device manager state related StateDB
// methods.
func TestStateDB_DriverManager(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// Getting nonexistent state should return nils
		ps, err := db.GetDriverPluginState()
		require.NoError(err)
		require.Nil(ps)

		// Putting PluginState should work
		state := &driverstate.PluginState{}
		require.NoError(db.PutDriverPluginState(state))

		// Getting should return the available state
		ps, err = db.GetDriverPluginState()
		require.NoError(err)
		require.NotNil(ps)
		require.Equal(state, ps)
	})
}

// TestStateDB_DynamicRegistry asserts the behavior of dynamic registry state related StateDB
// methods.
func TestStateDB_DynamicRegistry(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require := require.New(t)

		// Getting nonexistent state should return nils
		ps, err := db.GetDynamicPluginRegistryState()
		require.NoError(err)
		require.Nil(ps)

		// Putting PluginState should work
		state := &dynamicplugins.RegistryState{}
		require.NoError(db.PutDynamicPluginRegistryState(state))

		// Getting should return the available state
		ps, err = db.GetDynamicPluginRegistryState()
		require.NoError(err)
		require.NotNil(ps)
		require.Equal(state, ps)
	})
}

func TestStateDB_CheckResult_keyForCheck(t *testing.T) {
	ci.Parallel(t)

	allocID := "alloc1"
	checkID := structs.CheckID("id1")
	result := keyForCheck(allocID, checkID)
	exp := allocID + "_" + string(checkID)
	must.Eq(t, exp, string(result))
}

func TestStateDB_CheckResult(t *testing.T) {
	ci.Parallel(t)

	qr := func(id string) *structs.CheckQueryResult {
		return &structs.CheckQueryResult{
			ID:        structs.CheckID(id),
			Mode:      "healthiness",
			Status:    "passing",
			Output:    "nomad: tcp ok",
			Timestamp: 1,
			Group:     "group",
			Task:      "task",
			Service:   "service",
			Check:     "check",
		}
	}

	testDB(t, func(t *testing.T, db StateDB) {
		t.Run("put and get", func(t *testing.T) {
			err := db.PutCheckResult("alloc1", qr("abc123"))
			must.NoError(t, err)
			results, err := db.GetCheckResults()
			must.NoError(t, err)
			must.MapContainsKeys(t, results, []string{"alloc1"})
			must.MapContainsKeys(t, results["alloc1"], []structs.CheckID{"abc123"})
		})
	})

	testDB(t, func(t *testing.T, db StateDB) {
		t.Run("delete", func(t *testing.T) {
			must.NoError(t, db.PutCheckResult("alloc1", qr("id1")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id2")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id3")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id4")))
			must.NoError(t, db.PutCheckResult("alloc2", qr("id5")))
			err := db.DeleteCheckResults("alloc1", []structs.CheckID{"id2", "id3"})
			must.NoError(t, err)
			results, err := db.GetCheckResults()
			must.NoError(t, err)
			must.MapContainsKeys(t, results, []string{"alloc1", "alloc2"})
			must.MapContainsKeys(t, results["alloc1"], []structs.CheckID{"id1", "id4"})
			must.MapContainsKeys(t, results["alloc2"], []structs.CheckID{"id5"})
		})
	})

	testDB(t, func(t *testing.T, db StateDB) {
		t.Run("purge", func(t *testing.T) {
			must.NoError(t, db.PutCheckResult("alloc1", qr("id1")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id2")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id3")))
			must.NoError(t, db.PutCheckResult("alloc1", qr("id4")))
			must.NoError(t, db.PutCheckResult("alloc2", qr("id5")))
			err := db.PurgeCheckResults("alloc1")
			must.NoError(t, err)
			results, err := db.GetCheckResults()
			must.NoError(t, err)
			must.MapContainsKeys(t, results, []string{"alloc2"})
			must.MapContainsKeys(t, results["alloc2"], []structs.CheckID{"id5"})
		})
	})

}

// TestStateDB_Upgrade asserts calling Upgrade on new databases always
// succeeds.
func TestStateDB_Upgrade(t *testing.T) {
	ci.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require.NoError(t, db.Upgrade())
	})
}
