package state

import (
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func setupBoltStateDB(t *testing.T) (*BoltStateDB, func()) {
	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)

	db, err := NewBoltStateDB(testlog.HCLogger(t), dir)
	if err != nil {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error removing boltdb dir: %v", err)
		}
		t.Fatalf("error creating boltdb: %v", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("error closing boltdb: %v", err)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error removing boltdb dir: %v", err)
		}
	}

	return db.(*BoltStateDB), cleanup
}

func testDB(t *testing.T, f func(*testing.T, StateDB)) {
	boltdb, cleanup := setupBoltStateDB(t)
	defer cleanup()

	memdb := NewMemDB(testlog.HCLogger(t))

	impls := []StateDB{boltdb, memdb}

	for _, db := range impls {
		db := db
		t.Run(db.Name(), func(t *testing.T) {
			f(t, db)
		})
	}
}

// TestStateDB_Allocations asserts the behavior of GetAllAllocations, PutAllocation, and
// DeleteAllocationBucket for all operational StateDB implementations.
func TestStateDB_Allocations(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// Empty database should return empty non-nil results
		allocs, errs, err := db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, allocs)
		require.Empty(t, allocs)
		require.NotNil(t, errs)
		require.Empty(t, errs)

		// Put allocations
		alloc1 := mock.Alloc()
		alloc2 := mock.BatchAlloc()

		require.NoError(t, db.PutAllocation(alloc1))
		require.NoError(t, db.PutAllocation(alloc2))

		// Retrieve them
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, allocs)
		require.Len(t, allocs, 2)
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
		require.NotNil(t, errs)
		require.Empty(t, errs)

		// Add another
		alloc3 := mock.SystemAlloc()
		require.NoError(t, db.PutAllocation(alloc3))
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, allocs)
		require.Len(t, allocs, 3)
		require.Contains(t, allocs, alloc1)
		require.Contains(t, allocs, alloc2)
		require.Contains(t, allocs, alloc3)
		require.NotNil(t, errs)
		require.Empty(t, errs)

		// Deleting a nonexistent alloc is a noop
		require.NoError(t, db.DeleteAllocationBucket("asdf"))
		allocs, _, err = db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, allocs)
		require.Len(t, allocs, 3)

		// Delete alloc1
		require.NoError(t, db.DeleteAllocationBucket(alloc1.ID))
		allocs, errs, err = db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, allocs)
		require.Len(t, allocs, 2)
		require.Contains(t, allocs, alloc2)
		require.Contains(t, allocs, alloc3)
		require.NotNil(t, errs)
		require.Empty(t, errs)
	})
}

// Integer division, rounded up.
func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

// TestStateDB_Batch asserts the behavior of PutAllocation, PutNetworkStatus and
// DeleteAllocationBucket in batch mode, for all operational StateDB implementations.
func TestStateDB_Batch(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// For BoltDB, get initial tx_id
		var getTxID func() int
		var prevTxID int
		var batchDelay time.Duration
		var batchSize int
		if boltStateDB, ok := db.(*BoltStateDB); ok {
			boltdb := boltStateDB.DB().BoltDB()
			getTxID = func() int {
				tx, err := boltdb.Begin(true)
				require.NoError(t, err)
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
				require.NoError(t, db.PutNetworkStatus(alloc.ID, mock.AllocNetworkStatus(), WithBatchMode()))
				require.NoError(t, db.PutAllocation(alloc, WithBatchMode()))
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
			require.LessOrEqual(t, numTransactions, expectedNumTransactions)
			prevTxID = getTxID()
		}

		// Retrieve allocs and make sure they are the same (order can differ)
		readAllocs, errs, err := db.GetAllAllocations()
		require.NoError(t, err)
		require.NotNil(t, readAllocs)
		require.Len(t, readAllocs, len(allocs))
		require.NotNil(t, errs)
		require.Empty(t, errs)

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
				require.NoError(t, db.DeleteAllocationBucket(alloc.ID, WithBatchMode()))
				wg.Done()
			}(alloc)
		}
		wg.Wait()

		// Check BoltDB combined DeleteAllocationBucket calls into much fewer transactions.
		if getTxID != nil {
			numTransactions := getTxID() - prevTxID
			writeTime := time.Now().Sub(startTime)
			expectedNumTransactions := ceilDiv(numAllocs, batchSize) + ceilDiv(int(writeTime), int(batchDelay))
			require.LessOrEqual(t, numTransactions, expectedNumTransactions)
			prevTxID = getTxID()
		}

		// Check all allocs were deleted.
		readAllocs, errs, err = db.GetAllAllocations()
		require.NoError(t, err)
		require.Empty(t, readAllocs)
		require.Empty(t, errs)
	})
}

// TestStateDB_TaskState asserts the behavior of task state related StateDB
// methods.
func TestStateDB_TaskState(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// Getting nonexistent state should return nils
		ls, ts, err := db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(t, err)
		require.Nil(t, ls)
		require.Nil(t, ts)

		// Putting TaskState without first putting the allocation should work
		state := structs.NewTaskState()
		state.Failed = true // set a non-default value
		require.NoError(t, db.PutTaskState("allocid", "taskname", state))

		// Getting should return the available state
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(t, err)
		require.Nil(t, ls)
		require.Equal(t, state, ts)

		// Deleting a nonexistent task should not error
		require.NoError(t, db.DeleteTaskBucket("adsf", "asdf"))
		require.NoError(t, db.DeleteTaskBucket("asllocid", "asdf"))

		// Data should be untouched
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(t, err)
		require.Nil(t, ls)
		require.Equal(t, state, ts)

		// Deleting the task should remove the state
		require.NoError(t, db.DeleteTaskBucket("allocid", "taskname"))
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(t, err)
		require.Nil(t, ls)
		require.Nil(t, ts)

		// Putting LocalState should work just like TaskState
		origLocalState := trstate.NewLocalState()
		require.NoError(t, db.PutTaskRunnerLocalState("allocid", "taskname", origLocalState))
		ls, ts, err = db.GetTaskRunnerState("allocid", "taskname")
		require.NoError(t, err)
		require.Equal(t, origLocalState, ls)
		require.Nil(t, ts)
	})
}

// TestStateDB_DeviceManager asserts the behavior of device manager state related StateDB
// methods.
func TestStateDB_DeviceManager(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// Getting nonexistent state should return nils
		ps, err := db.GetDevicePluginState()
		require.NoError(t, err)
		require.Nil(t, ps)

		// Putting PluginState should work
		state := &dmstate.PluginState{}
		require.NoError(t, db.PutDevicePluginState(state))

		// Getting should return the available state
		ps, err = db.GetDevicePluginState()
		require.NoError(t, err)
		require.NotNil(t, ps)
		require.Equal(t, state, ps)
	})
}

// TestStateDB_DriverManager asserts the behavior of device manager state related StateDB
// methods.
func TestStateDB_DriverManager(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// Getting nonexistent state should return nils
		ps, err := db.GetDriverPluginState()
		require.NoError(t, err)
		require.Nil(t, ps)

		// Putting PluginState should work
		state := &driverstate.PluginState{}
		require.NoError(t, db.PutDriverPluginState(state))

		// Getting should return the available state
		ps, err = db.GetDriverPluginState()
		require.NoError(t, err)
		require.NotNil(t, ps)
		require.Equal(t, state, ps)
	})
}

// TestStateDB_DynamicRegistry asserts the behavior of dynamic registry state related StateDB
// methods.
func TestStateDB_DynamicRegistry(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {

		// Getting nonexistent state should return nils
		ps, err := db.GetDynamicPluginRegistryState()
		require.NoError(t, err)
		require.Nil(t, ps)

		// Putting PluginState should work
		state := &dynamicplugins.RegistryState{}
		require.NoError(t, db.PutDynamicPluginRegistryState(state))

		// Getting should return the available state
		ps, err = db.GetDynamicPluginRegistryState()
		require.NoError(t, err)
		require.NotNil(t, ps)
		require.Equal(t, state, ps)
	})
}

// TestStateDB_Upgrade asserts calling Upgrade on new databases always
// succeeds.
func TestStateDB_Upgrade(t *testing.T) {
	testutil.Parallel(t)

	testDB(t, func(t *testing.T, db StateDB) {
		require.NoError(t, db.Upgrade())
	})
}
