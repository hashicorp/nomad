package state

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
	"golang.org/x/crypto/blake2b"
)

type kvStore interface {
	Path() []byte
	Bucket(name []byte) kvStore
	CreateBucket(key []byte) (kvStore, error)
	CreateBucketIfNotExists(key []byte) (kvStore, error)
	DeleteBucket(key []byte) error
	Get(key []byte) (val []byte)
	Put(key, val []byte) error
}

// keyValueCodec handles encoding and decoding values from a key/value store
// such as boltdb.
type keyValueCodec struct {
	// hashes maps buckets to keys to the hash of the last content written:
	//   bucket -> key -> hash  for example:
	//   allocations/1234       -> alloc      -> abcd
	//   allocations/1234/redis -> task_state -> efff
	hashes     map[string]map[string][]byte
	hashesLock sync.Mutex
}

func newKeyValueCodec() *keyValueCodec {
	return &keyValueCodec{
		hashes: make(map[string]map[string][]byte),
	}
}

// Put into kv store iff it has changed since the last write. A globally
// unique key is constructed for each value by concatinating the path and key
// passed in.
func (c *keyValueCodec) Put(bkt kvStore, key []byte, val interface{}) error {
	// buffer for writing serialized state to
	var buf bytes.Buffer

	// Hash for skipping unnecessary writes
	h, err := blake2b.New256(nil)
	if err != nil {
		// Programming error that should never happen!
		return err
	}

	// Multiplex writes to both hasher and buffer
	w := io.MultiWriter(h, &buf)

	// Serialize the object
	if err := codec.NewEncoder(w, structs.MsgpackHandle).Encode(val); err != nil {
		return fmt.Errorf("failed to encode passed object: %v", err)
	}

	// If the hashes are equal, skip the write
	hashPath := string(bkt.Path())
	hashKey := string(key)
	hashVal := h.Sum(nil)

	// lastHash value or nil if it hasn't been hashed yet
	var lastHash []byte

	c.hashesLock.Lock()
	if hashBkt, ok := c.hashes[hashPath]; ok {
		lastHash = hashBkt[hashKey]
	} else {
		// Create hash bucket
		c.hashes[hashPath] = make(map[string][]byte, 2)
	}
	c.hashesLock.Unlock()

	if bytes.Equal(hashVal, lastHash) {
		return nil
	}

	// New value: write it to the underlying store
	if err := bkt.Put(key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write data at key %s: %v", key, err)
	}

	// New value written, store hash (bucket path map was created above)
	c.hashesLock.Lock()
	c.hashes[hashPath][hashKey] = hashVal
	c.hashesLock.Unlock()

	return nil

}

// Get value by key from boltdb.
func (c *keyValueCodec) Get(bkt kvStore, key []byte, obj interface{}) error {
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

// DeleteBucket or do nothing if bucket doesn't exist.
func (c *keyValueCodec) DeleteBucket(parent kvStore, bktName []byte) error {
	// Get the path of the bucket being deleted
	bkt := parent.Bucket(bktName)
	if bkt == nil {
		// Doesn't exist! Nothing to delete
		return nil
	}

	// Delete the bucket
	err := parent.DeleteBucket(bktName)

	// Always purge all corresponding hashes to prevent memory leaks
	c.hashesLock.Lock()
	delete(c.hashes, string(bkt.Path()))
	c.hashesLock.Unlock()

	return err
}
