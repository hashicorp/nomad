package client

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

// getAllocationBucket returns the bucket used to persist state about a
// particular allocation. If the root allocation bucket or the specific
// allocation bucket doesn't exist, it will be created.
func getAllocationBucket(tx *bolt.Tx, allocID string) (*bolt.Bucket, error) {
	if !tx.Writable() {
		return nil, fmt.Errorf("transaction must be writable")
	}

	// Retrieve the root allocations bucket
	allocations, err := tx.CreateBucketIfNotExists(allocationsBucket)
	if err != nil {
		return nil, err
	}

	// Retrieve the specific allocations bucket
	alloc, err := allocations.CreateBucketIfNotExists([]byte(allocID))
	if err != nil {
		return nil, err
	}

	return alloc, nil
}

// getTaskBucket returns the bucket used to persist state about a
// particular task. If the root allocation bucket, the specific
// allocation or task bucket doesn't exist, they will be created.
func getTaskBucket(tx *bolt.Tx, allocID, taskName string) (*bolt.Bucket, error) {
	if !tx.Writable() {
		return nil, fmt.Errorf("transaction must be writable")
	}

	// Retrieve the root allocations bucket
	allocations, err := tx.CreateBucketIfNotExists(allocationsBucket)
	if err != nil {
		return nil, err
	}

	// Retrieve the specific allocations bucket
	alloc, err := allocations.CreateBucketIfNotExists([]byte(allocID))
	if err != nil {
		return nil, err
	}

	// Retrieve the specific task bucket
	task, err := alloc.CreateBucketIfNotExists([]byte(taskName))
	if err != nil {
		return nil, err
	}

	return task, nil
}
