package state

import (
	"fmt"
	"path/filepath"

	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/helper/boltdd"
	"github.com/hashicorp/nomad/nomad/structs"
)

/*
The client has a boltDB backed state store. The schema as of 0.6 looks as follows:

allocations/ (bucket)
|--> <alloc-id>/ (bucket)
    |--> alloc -> *structs.Allocation
    |--> alloc_runner persisted objects (k/v)
	|--> <task-name>/ (bucket)
        |--> task_runner persisted objects (k/v)
*/

var (
	// allocationsBucket is the bucket name containing all allocation related
	// data
	allocationsBucket = []byte("allocations")

	// allocKey is the key serialized Allocations are stored under
	allocKey = []byte("alloc")

	// taskRunnerStateAllKey holds all the task runners state. At the moment
	// there is no need to split it
	//XXX Old key - going to need to migrate
	//taskRunnerStateAllKey = []byte("simple-all")

	// allocations -> $allocid -> $taskname -> the keys below
	taskLocalStateKey = []byte("local_state")
	taskStateKey      = []byte("task_state")
)

// NewStateDBFunc creates a StateDB given a state directory.
type NewStateDBFunc func(stateDir string) (StateDB, error)

// GetStateDBFactory returns a func for creating a StateDB
func GetStateDBFactory(devMode bool) NewStateDBFunc {
	// Return a noop state db implementation when in debug mode
	if devMode {
		return func(string) (StateDB, error) {
			return NoopDB{}, nil
		}
	}

	return NewBoltStateDB
}

// BoltStateDB persists and restores Nomad client state in a boltdb. All
// methods are safe for concurrent access.
type BoltStateDB struct {
	db *boltdd.DB
}

// NewBoltStateDB creates or opens an existing boltdb state file or returns an
// error.
func NewBoltStateDB(stateDir string) (StateDB, error) {
	// Create or open the boltdb state database
	db, err := boltdd.Open(filepath.Join(stateDir, "state.db"), 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create state database: %v", err)
	}

	sdb := &BoltStateDB{
		db: db,
	}
	return sdb, nil
}

// GetAllAllocations gets all allocations persisted by this client and returns
// a map of alloc ids to errors for any allocations that could not be restored.
//
// If a fatal error was encountered it will be returned and the other two
// values will be nil.
func (s *BoltStateDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	var allocs []*structs.Allocation
	var errs map[string]error
	err := s.db.View(func(tx *boltdd.Tx) error {
		allocs, errs = s.getAllAllocations(tx)
		return nil
	})

	// db.View itself may return an error, so still check
	if err != nil {
		return nil, nil, err
	}

	return allocs, errs, nil
}

// allocEntry wraps values in the Allocations buckets
type allocEntry struct {
	Alloc *structs.Allocation
}

func (s *BoltStateDB) getAllAllocations(tx *boltdd.Tx) ([]*structs.Allocation, map[string]error) {
	allocationsBkt := tx.Bucket(allocationsBucket)
	if allocationsBkt == nil {
		// No allocs
		return nil, nil
	}

	var allocs []*structs.Allocation
	errs := map[string]error{}

	// Create a cursor for iteration.
	c := allocationsBkt.BoltBucket().Cursor()

	// Iterate over all the allocation buckets
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		allocID := string(k)
		allocBkt := allocationsBkt.Bucket(k)
		if allocBkt == nil {
			errs[allocID] = fmt.Errorf("missing alloc bucket")
			continue
		}

		var ae allocEntry
		if err := allocBkt.Get(allocKey, &ae); err != nil {
			errs[allocID] = fmt.Errorf("failed to decode alloc: %v", err)
			continue
		}

		allocs = append(allocs, ae.Alloc)
	}

	return allocs, errs
}

// PutAllocation stores an allocation or returns an error.
func (s *BoltStateDB) PutAllocation(alloc *structs.Allocation) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocsBkt, err := tx.CreateBucketIfNotExists(allocationsBucket)
		if err != nil {
			return err
		}

		// Retrieve the specific allocations bucket
		key := []byte(alloc.ID)
		allocBkt, err := allocsBkt.CreateBucketIfNotExists(key)
		if err != nil {
			return err
		}

		allocState := allocEntry{
			Alloc: alloc,
		}
		return allocBkt.Put(allocKey, &allocState)
	})
}

// GetTaskRunnerState restores TaskRunner specific state. An error will be
// returned if the alloc or task could not be found.
//
//FIXME make this the desired behavior:
// - if task bucket doesn't exist: return all nils
// - getTaskBucket *here* should be readonly (do not try to create if it doesn't exist)
// - any other getTaskBucket errors should be returned
//
// - Get(taskLocalStateKey) -> if key doesn't exist, return nil
//                          -> other errors should be returned
// - Get(taskStateKey) -> if key doesn't exist, return nil
//                     -> other errors should be returned
//TODO make other Getters behave similarly
//TODO make sure LocalState.Hooks map is non-nil *in TR.Restore*
func (s *BoltStateDB) GetTaskRunnerState(allocID, taskName string) (*trstate.LocalState, *structs.TaskState, error) {
	var ls trstate.LocalState
	var ts structs.TaskState

	err := s.db.View(func(tx *boltdd.Tx) error {
		bkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to get task %q bucket: %v", taskName, err)
		}

		// Restore Local State
		if err := bkt.Get(taskLocalStateKey, &ls); err != nil {
			return fmt.Errorf("failed to read local task runner state: %v", err)
		}

		// Restore Task State
		if err := bkt.Get(taskStateKey, &ts); err != nil {
			return fmt.Errorf("failed to read task state: %v", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return &ls, &ts, nil
}

// PutTaskRunnerLocalState stores TaskRunner's LocalState or returns an error.
func (s *BoltStateDB) PutTaskRunnerLocalState(allocID, taskName string, val *trstate.LocalState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		taskBkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		if err := taskBkt.Put(taskLocalStateKey, val); err != nil {
			return fmt.Errorf("failed to write task_runner state: %v", err)
		}

		return nil
	})
}

// PutTaskState stores a task's state or returns an error.
func (s *BoltStateDB) PutTaskState(allocID, taskName string, state *structs.TaskState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		taskBkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		return taskBkt.Put(taskStateKey, state)
	})
}

// DeleteTaskBucket is used to delete a task bucket if it exists.
func (s *BoltStateDB) DeleteTaskBucket(allocID, taskName string) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocations := tx.Bucket(allocationsBucket)
		if allocations == nil {
			return nil
		}

		// Retrieve the specific allocations bucket
		alloc := allocations.Bucket([]byte(allocID))
		if alloc == nil {
			return nil
		}

		// Check if the bucket exists
		key := []byte(taskName)
		return alloc.DeleteBucket(key)
	})
}

// DeleteAllocationBucket is used to delete an allocation bucket if it exists.
func (s *BoltStateDB) DeleteAllocationBucket(allocID string) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocations := tx.Bucket(allocationsBucket)
		if allocations == nil {
			return nil
		}

		key := []byte(allocID)
		return allocations.DeleteBucket(key)
	})
}

// Close releases all database resources and unlocks the database file on disk.
// All transactions must be closed before closing the database.
func (s *BoltStateDB) Close() error {
	return s.db.Close()
}

// getAllocationBucket returns the bucket used to persist state about a
// particular allocation. If the root allocation bucket or the specific
// allocation bucket doesn't exist, it will be created as long as the
// transaction is writable.
func getAllocationBucket(tx *boltdd.Tx, allocID string) (*boltdd.Bucket, error) {
	var err error
	w := tx.Writable()

	// Retrieve the root allocations bucket
	allocations := tx.Bucket(allocationsBucket)
	if allocations == nil {
		if !w {
			return nil, fmt.Errorf("Allocations bucket doesn't exist and transaction is not writable")
		}

		allocations, err = tx.CreateBucketIfNotExists(allocationsBucket)
		if err != nil {
			return nil, err
		}
	}

	// Retrieve the specific allocations bucket
	key := []byte(allocID)
	alloc := allocations.Bucket(key)
	if alloc == nil {
		if !w {
			return nil, fmt.Errorf("Allocation bucket doesn't exist and transaction is not writable")
		}

		alloc, err = allocations.CreateBucket(key)
		if err != nil {
			return nil, err
		}
	}

	return alloc, nil
}

// getTaskBucket returns the bucket used to persist state about a
// particular task. If the root allocation bucket, the specific
// allocation or task bucket doesn't exist, they will be created as long as the
// transaction is writable.
func getTaskBucket(tx *boltdd.Tx, allocID, taskName string) (*boltdd.Bucket, error) {
	alloc, err := getAllocationBucket(tx, allocID)
	if err != nil {
		return nil, err
	}

	// Retrieve the specific task bucket
	w := tx.Writable()
	key := []byte(taskName)
	task := alloc.Bucket(key)
	if task == nil {
		if !w {
			return nil, fmt.Errorf("Task bucket doesn't exist and transaction is not writable")
		}

		task, err = alloc.CreateBucket(key)
		if err != nil {
			return nil, err
		}
	}

	return task, nil
}
