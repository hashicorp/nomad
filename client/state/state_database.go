package state

import (
	"fmt"
	"path/filepath"

	"github.com/boltdb/bolt"
	trstate "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
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
			return noopDB{}, nil
		}
	}

	return NewBoltStateDB
}

// BoltStateDB persists and restores Nomad client state in a boltdb. All
// methods are safe for concurrent access. Create via NewStateDB by setting
// devMode=false.
type BoltStateDB struct {
	db    *bolt.DB
	codec *keyValueCodec
}

func NewBoltStateDB(stateDir string) (StateDB, error) {
	// Create or open the boltdb state database
	db, err := bolt.Open(filepath.Join(stateDir, "state.db"), 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create state database: %v", err)
	}

	sdb := &BoltStateDB{
		db:    db,
		codec: newKeyValueCodec(),
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
	err := s.db.View(func(tx *bolt.Tx) error {
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

func (s *BoltStateDB) getAllAllocations(tx *bolt.Tx) ([]*structs.Allocation, map[string]error) {
	allocationsBkt := newNamedBucket(tx, allocationsBucket)
	if allocationsBkt == nil {
		// No allocs
		return nil, nil
	}

	var allocs []*structs.Allocation
	errs := map[string]error{}

	// Create a cursor for iteration.
	c := allocationsBkt.bkt.Cursor()

	// Iterate over all the allocation buckets
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		allocID := string(k)
		allocBkt := allocationsBkt.Bucket(k)
		if allocBkt == nil {
			errs[allocID] = fmt.Errorf("missing alloc bucket")
			continue
		}

		var allocState allocEntry
		if err := s.codec.Get(allocBkt, allocKey, &allocState); err != nil {
			errs[allocID] = fmt.Errorf("failed to decode alloc %v", err)
			continue
		}

		allocs = append(allocs, allocState.Alloc)
	}

	return allocs, errs
}

// PutAllocation stores an allocation or returns an error.
func (s *BoltStateDB) PutAllocation(alloc *structs.Allocation) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Retrieve the root allocations bucket
		allocsBkt, err := createNamedBucketIfNotExists(tx, allocationsBucket)
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
		return s.codec.Put(allocBkt, allocKey, &allocState)
	})
}

// GetTaskRunnerState restores TaskRunner specific state.
//TODO wrap in a single struct?
func (s *BoltStateDB) GetTaskRunnerState(allocID, taskName string) (*trstate.LocalState, *structs.TaskState, error) {
	var ls trstate.LocalState
	var ts structs.TaskState

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to get task %q bucket: %v", taskName, err)
		}

		// Restore Local State
		//XXX set persisted hash to avoid immediate write on first use?
		if err := s.codec.Get(bkt, taskLocalStateKey, &ls); err != nil {
			return fmt.Errorf("failed to read local task runner state: %v", err)
		}

		// Restore Task State
		if err := s.codec.Get(bkt, taskStateKey, &ts); err != nil {
			return fmt.Errorf("failed to read task state: %v", err)
		}

		// XXX if driver has task {
		// tr.restoreDriver()
		// }

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return &ls, &ts, nil

}

// PutTaskRunnerLocalState stores TaskRunner's LocalState or returns an error.
// It is up to the caller to serialize the state to bytes.
func (s *BoltStateDB) PutTaskRunnerLocalState(allocID, taskName string, val interface{}) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		taskBkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		if err := s.codec.Put(taskBkt, taskLocalStateKey, val); err != nil {
			return fmt.Errorf("failed to write task_runner state: %v", err)
		}

		return nil
	})
}

// PutTaskState stores a task's state or returns an error.
func (s *BoltStateDB) PutTaskState(allocID, taskName string, state *structs.TaskState) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		taskBkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		return s.codec.Put(taskBkt, taskStateKey, state)
	})
}

// DeleteTaskBucket is used to delete a task bucket if it exists.
func (s *BoltStateDB) DeleteTaskBucket(allocID, taskName string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Retrieve the root allocations bucket
		allocations := newNamedBucket(tx, allocationsBucket)
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
		return s.codec.DeleteBucket(alloc, key)
	})
}

// DeleteAllocationBucket is used to delete an allocation bucket if it exists.
func (s *BoltStateDB) DeleteAllocationBucket(allocID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Retrieve the root allocations bucket
		allocations := newNamedBucket(tx, allocationsBucket)
		if allocations == nil {
			return nil
		}

		key := []byte(allocID)
		return s.codec.DeleteBucket(allocations, key)
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
func getAllocationBucket(tx *bolt.Tx, allocID string) (kvStore, error) {
	var err error
	w := tx.Writable()

	// Retrieve the root allocations bucket
	allocations := newNamedBucket(tx, allocationsBucket)
	if allocations == nil {
		if !w {
			return nil, fmt.Errorf("Allocations bucket doesn't exist and transaction is not writable")
		}

		allocations, err = createNamedBucketIfNotExists(tx, allocationsBucket)
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
func getTaskBucket(tx *bolt.Tx, allocID, taskName string) (kvStore, error) {
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
