package state

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgraph-io/badger"
	badgeropt "github.com/dgraph-io/badger/options"
	trstate "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

/*
The client has a Badger backed state store. The schema as of 0.6 looks as follows:

allocations/ (bucket)
|--> <alloc-id>/ (bucket)
    |--> alloc -> *structs.Allocation
    |--> alloc_runner persisted objects (k/v)
	|--> <task-name>/ (bucket)
        |--> task_runner persisted objects (k/v)
*/

// BadgerStateDB persists and restores Nomad client state in a boltdb. All
// methods are safe for concurrent access.
type BadgerStateDB struct {
	db *badger.DB
}

// NewBadgerStateDB creates or opens an existing boltdb state file or returns an
// error.
func NewBadgerStateDB(stateDir string) (StateDB, error) {
	opts := badger.DefaultOptions
	opts.Dir = stateDir
	opts.ValueDir = stateDir
	opts.ValueLogLoadingMode = badgeropt.FileIO //no need for (default) mmap here

	// Create or open the boltdb state database
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create state database: %v", err)
	}

	sdb := &BadgerStateDB{
		db: db,
	}
	return sdb, nil
}

// GetAllAllocations gets all allocations persisted by this client and returns
// a map of alloc ids to errors for any allocations that could not be restored.
//
// If a fatal error was encountered it will be returned and the other two
// values will be nil.
func (s *BadgerStateDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	var allocs []*structs.Allocation
	var errs map[string]error
	err := s.db.View(func(tx *badger.Txn) (e error) {
		allocs, errs, e = s.getAllAllocations(tx)
		return nil
	})

	// db.View itself may return an error, so still check
	if err != nil {
		return nil, nil, err
	}

	return allocs, errs, nil
}

func (s *BadgerStateDB) getAllAllocations(tx *badger.Txn) ([]*structs.Allocation, map[string]error, error) {
	var allocs []*structs.Allocation
	errs := map[string]error{}

	keySeparator := "/"
	allocKeyPrefix := string(allocationsBucket) + keySeparator
	allocKeySuffix := keySeparator + string(allocKey)

	err := s.db.View(func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := tx.NewIterator(opts)
		defer it.Close()
		for it.Seek([]byte(allocKeyPrefix)); it.ValidForPrefix([]byte(allocKeyPrefix)); it.Next() {
			item := it.Item()

			s := string(item.Key())
			if !strings.HasSuffix(s, allocKeySuffix) {
				continue
			}

			allocID := strings.TrimSuffix(strings.TrimPrefix(s, allocKeyPrefix), allocKeySuffix)
			b, err := item.Value()
			if err != nil {
				errs[allocID] = fmt.Errorf("failed to decode alloc: %v", err)
				continue
			}

			var ae allocEntry
			if err := json.Unmarshal(b, &ae); err != nil {
				errs[allocID] = fmt.Errorf("failed to decode alloc: %v", err)
				continue
			}

			allocs = append(allocs, ae.Alloc)
		}
		return nil
	})

	return allocs, errs, err
}

// PutAllocation stores an allocation or returns an error.
func (s *BadgerStateDB) PutAllocation(alloc *structs.Allocation) error {
	key := allocKeyFnc(alloc.ID)
	return s.putKey(key, allocEntry{
		Alloc: alloc,
	})
}

func allocKeyFnc(id string) string {
	return fmt.Sprintf("%s/%s/%s", string(allocationsBucket), id, allocKey)
}

// GetTaskRunnerState restores TaskRunner specific state.
func (s *BadgerStateDB) GetTaskRunnerState(allocID, taskName string) (retLs *trstate.LocalState, retTs *structs.TaskState, err error) {
	taskPrefix := taskKeyPrefix(allocID, taskName)
	keySeparator := "/"
	taskLocalStateKeySuffix := keySeparator + string(taskLocalStateKey)
	taskStateKeySuffix := keySeparator + string(taskStateKey)

	var foundAlloc bool
	err = s.db.View(func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := tx.NewIterator(opts)
		defer it.Close()

		allocPrefix := allocKeyFnc(allocID)
		for it.Seek([]byte(allocPrefix)); it.ValidForPrefix([]byte(allocPrefix)); it.Next() {
			foundAlloc = true
		}

		for it.Seek([]byte(taskPrefix)); it.ValidForPrefix([]byte(taskPrefix)); it.Next() {
			item := it.Item()
			s := string(item.Key())
			if strings.HasSuffix(s, taskLocalStateKeySuffix) {
				b, err := item.Value()
				if err != nil {
					return fmt.Errorf("failed to read local task runner state: %v", err)
				}
				var ls trstate.LocalState
				if err := json.Unmarshal(b, &ls); err != nil {
					return err
				}
				retLs = &ls
			} else if strings.HasSuffix(s, taskStateKeySuffix) {
				b, err := item.Value()
				if err != nil {
					return fmt.Errorf("failed to read task state: %v", err)
				}
				var ts structs.TaskState
				if err := json.Unmarshal(b, &ts); err != nil {
					return fmt.Errorf("failed to read task state: %v", err)
				}
				retTs = &ts
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	if !foundAlloc {
		return retLs, retTs, fmt.Errorf("failed to get task \"%s\" bucket: Allocations bucket doesn't exist and transaction is not writable", taskName)
	}
	if retTs == nil {
		return retLs, retTs, fmt.Errorf("failed to read task state: no data at key %s", taskStateKey)
	}
	if retLs == nil {
		return retLs, retTs, fmt.Errorf("failed to read local task runner state: no data at key %s", taskLocalStateKey)
	}

	return retLs, retTs, nil
}

// PutTaskRunnerLocalState stores TaskRunner's LocalState or returns an error.
func (s *BadgerStateDB) PutTaskRunnerLocalState(allocID, taskName string, val interface{}) error {
	key := taskKeyChild(allocID, taskName, taskLocalStateKey)
	//allocations -> $allocid -> $taskname -> taskLocalStateKey
	return s.putKey(key, val)
}

func taskKeyChild(allocID, taskName string, childKey []byte) string {
	return fmt.Sprintf("%s/%s/%s/%s", string(allocationsBucket), allocID, taskName, string(childKey))
}

func taskKeyPrefix(allocID, taskName string) string {
	return fmt.Sprintf("%s/%s/%s/", string(allocationsBucket), allocID, taskName)
}

// PutTaskState stores a task's state or returns an error.
func (s *BadgerStateDB) PutTaskState(allocID, taskName string, state *structs.TaskState) error {
	//allocations -> $allocid -> $taskname -> taskStateKey
	key := taskKeyChild(allocID, taskName, taskStateKey)
	return s.putKey(key, state)
}

// DeleteTaskBucket is used to delete a task bucket if it exists.
func (s *BadgerStateDB) DeleteTaskBucket(allocID, taskName string) error {
	//allocations -> $allocid -> $taskname
	prefix := taskKeyPrefix(allocID, taskName)
	return s.deletePrefix(prefix)
}

// DeleteAllocationBucket is used to delete an allocation bucket if it exists.
func (s *BadgerStateDB) DeleteAllocationBucket(allocID string) error {
	//allocations -> $allocid -> $taskname
	prefix := fmt.Sprintf("%s/%s/", string(allocationsBucket), allocID)
	return s.deletePrefix(prefix)
}

func (s *BadgerStateDB) deletePrefix(prefix string) error {
	return s.db.Update(func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := tx.NewIterator(opts)
		defer it.Close()
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			if err := tx.Delete(item.Key()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BadgerStateDB) putKey(key string, val interface{}) error {
	return s.db.Update(func(tx *badger.Txn) error {
		allocStateBytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("failed to write task_runner state: %v", err)
		}

		return tx.Set([]byte(key), allocStateBytes)
	})
}

// Close releases all database resources and unlocks the database file on disk.
// All transactions must be closed before closing the database.
func (s *BadgerStateDB) Close() error {
	return s.db.Close()
}
