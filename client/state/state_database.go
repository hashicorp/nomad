package state

import (
	"bytes"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
)

/*
The client has a boltDB backed state store. The schema as of 0.6 looks as follows:

allocations/ (bucket)
|--> <alloc-id>/ (bucket)
    |--> alloc_runner persisted objects (k/v)
	|--> <task-name>/ (bucket)
        |--> task_runner persisted objects (k/v)
*/

var (
	// allocationsBucket is the bucket name containing all allocation related
	// data
	allocationsBucket = []byte("allocations")
)

func PutObject(bkt *bolt.Bucket, key []byte, obj interface{}) error {
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

func PutData(bkt *bolt.Bucket, key, value []byte) error {
	if !bkt.Writable() {
		return fmt.Errorf("bucket must be writable")
	}

	if err := bkt.Put(key, value); err != nil {
		return fmt.Errorf("failed to write data at key %v: %v", string(key), err)
	}

	return nil
}

func GetObject(bkt *bolt.Bucket, key []byte, obj interface{}) error {
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

// GetAllocationBucket returns the bucket used to persist state about a
// particular allocation. If the root allocation bucket or the specific
// allocation bucket doesn't exist, it will be created as long as the
// transaction is writable.
func GetAllocationBucket(tx *bolt.Tx, allocID string) (*bolt.Bucket, error) {
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

// GetTaskBucket returns the bucket used to persist state about a
// particular task. If the root allocation bucket, the specific
// allocation or task bucket doesn't exist, they will be created as long as the
// transaction is writable.
func GetTaskBucket(tx *bolt.Tx, allocID, taskName string) (*bolt.Bucket, error) {
	alloc, err := GetAllocationBucket(tx, allocID)
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

//XXX duplicated in arv2?!
var (
	// The following are the key paths written to the state database
	allocRunnerStateAllocKey = []byte("alloc")
)

type allocRunnerAllocState struct {
	Alloc *structs.Allocation
}

func GetAllAllocations(tx *bolt.Tx) ([]*structs.Allocation, error) {
	allocationsBkt := tx.Bucket(allocationsBucket)
	if allocationsBkt == nil {
		// No allocs
		return nil, nil
	}

	var allocs []*structs.Allocation

	// Create a cursor for iteration.
	c := allocationsBkt.Cursor()

	// Iterate over all the allocation buckets
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		allocID := string(k)
		allocBkt := allocationsBkt.Bucket(k)
		if allocBkt == nil {
			//XXX merr?
			return nil, fmt.Errorf("alloc %q missing", allocID)
		}

		var allocState allocRunnerAllocState
		if err := GetObject(allocBkt, allocRunnerStateAllocKey, &allocState); err != nil {
			//XXX merr?
			return nil, fmt.Errorf("failed to restore alloc %q: %v", allocID, err)
		}
		allocs = append(allocs, allocState.Alloc)
	}

	return allocs, nil
}

// PutAllocation stores an allocation given a writable transaction.
func PutAllocation(tx *bolt.Tx, alloc *structs.Allocation) error {
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

	allocState := allocRunnerAllocState{
		Alloc: alloc,
	}
	return PutObject(allocBkt, allocRunnerStateAllocKey, &allocState)
}
