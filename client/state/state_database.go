package state

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/boltdb/bolt"
	trstate "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
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

// DeleteAllocationBucket is used to delete an allocation bucket if it exists.
func DeleteAllocationBucket(tx *bolt.Tx, allocID string) error {
	if !tx.Writable() {
		return fmt.Errorf("transaction must be writable")
	}

	// Retrieve the root allocations bucket
	allocations := tx.Bucket(allocationsBucket)
	if allocations == nil {
		return nil
	}

	// Check if the bucket exists
	key := []byte(allocID)
	if allocBkt := allocations.Bucket(key); allocBkt == nil {
		return nil
	}

	return allocations.DeleteBucket(key)
}

// DeleteTaskBucket is used to delete a task bucket if it exists.
func DeleteTaskBucket(tx *bolt.Tx, allocID, taskName string) error {
	if !tx.Writable() {
		return fmt.Errorf("transaction must be writable")
	}

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
	if taskBkt := alloc.Bucket(key); taskBkt == nil {
		return nil
	}

	return alloc.DeleteBucket(key)
}

var ()

type allocEntry struct {
	Alloc *structs.Allocation
}

func getAllAllocations(tx *bolt.Tx) ([]*structs.Allocation, map[string]error) {
	allocationsBkt := tx.Bucket(allocationsBucket)
	if allocationsBkt == nil {
		// No allocs
		return nil, nil
	}

	var allocs []*structs.Allocation
	errs := map[string]error{}

	// Create a cursor for iteration.
	c := allocationsBkt.Cursor()

	// Iterate over all the allocation buckets
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		allocID := string(k)
		allocBkt := allocationsBkt.Bucket(k)
		if allocBkt == nil {
			errs[allocID] = fmt.Errorf("missing alloc bucket")
			continue
		}

		var allocState allocEntry
		if err := getObject(allocBkt, allocKey, &allocState); err != nil {
			errs[allocID] = fmt.Errorf("failed to decode alloc %v", err)
			continue
		}

		allocs = append(allocs, allocState.Alloc)
	}

	return allocs, errs
}

// BoltStateDB persists and restores Nomad client state in a boltdb. All
// methods are safe for concurrent access. Create via NewStateDB by setting
// devMode=false.
type BoltStateDB struct {
	db *bolt.DB
}

func NewStateDB(stateDir string, devMode bool) (StateDB, error) {
	// Return a noop state db implementation when in debug mode
	if devMode {
		return noopDB{}, nil
	}

	// Create or open the boltdb state database
	db, err := bolt.Open(filepath.Join(stateDir, "state.db"), 0600, nil)
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
	err := s.db.View(func(tx *bolt.Tx) error {
		allocs, errs = getAllAllocations(tx)
		return nil
	})

	// db.View itself may return an error, so still check
	if err != nil {
		return nil, nil, err
	}

	return allocs, errs, nil
}

// PutAllocation stores an allocation or returns an error.
func (s *BoltStateDB) PutAllocation(alloc *structs.Allocation) error {
	return s.db.Update(func(tx *bolt.Tx) error {
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
		return putObject(allocBkt, allocKey, &allocState)
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
		if err := getObject(bkt, taskLocalStateKey, &ls); err != nil {
			return fmt.Errorf("failed to read local task runner state: %v", err)
		}

		// Restore Task State
		if err := getObject(bkt, taskStateKey, &ts); err != nil {
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
func (s *BoltStateDB) PutTaskRunnerLocalState(allocID, taskName string, buf []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		taskBkt, err := getTaskBucket(tx, allocID, taskName)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		if err := putData(taskBkt, taskLocalStateKey, buf); err != nil {
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

		return putObject(taskBkt, taskStateKey, state)
	})
}

// Close releases all database resources and unlocks the database file on disk.
// All transactions must be closed before closing the database.
func (s *BoltStateDB) Close() error {
	return s.db.Close()
}

func putObject(bkt *bolt.Bucket, key []byte, obj interface{}) error {
	if !bkt.Writable() {
		return fmt.Errorf("bucket must be writable")
	}

	// Serialize the object
	var buf bytes.Buffer
	if err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(obj); err != nil {
		return fmt.Errorf("failed to encode passed object: %v", err)
	}

	if err := bkt.Put(key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write data at key %v: %v", string(key), err)
	}

	return nil
}

func putData(bkt *bolt.Bucket, key, value []byte) error {
	if !bkt.Writable() {
		return fmt.Errorf("bucket must be writable")
	}

	if err := bkt.Put(key, value); err != nil {
		return fmt.Errorf("failed to write data at key %v: %v", string(key), err)
	}

	return nil
}

func getObject(bkt *bolt.Bucket, key []byte, obj interface{}) error {
	// Get the data
	data := bkt.Get(key)
	if data == nil {
		return fmt.Errorf("no data at key %v", string(key))
	}

	// Deserialize the object
	if err := codec.NewDecoderBytes(data, structs.MsgpackHandle).Decode(obj); err != nil {
		return fmt.Errorf("failed to decode data into passed object: %v", err)
	}

	return nil
}

// getAllocationBucket returns the bucket used to persist state about a
// particular allocation. If the root allocation bucket or the specific
// allocation bucket doesn't exist, it will be created as long as the
// transaction is writable.
func getAllocationBucket(tx *bolt.Tx, allocID string) (*bolt.Bucket, error) {
	var err error
	w := tx.Writable()

	// Retrieve the root allocations bucket
	allocations := tx.Bucket(allocationsBucket)
	if allocations == nil {
		if !w {
			return nil, fmt.Errorf("Allocations bucket doesn't exist and transaction is not writable")
		}

		allocations, err = tx.CreateBucket(allocationsBucket)
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
func getTaskBucket(tx *bolt.Tx, allocID, taskName string) (*bolt.Bucket, error) {
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
