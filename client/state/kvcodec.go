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
	Get(key []byte) (val []byte)
	Put(key, val []byte) error
	Writable() bool
}

// keyValueCodec handles encoding and decoding values from a key/value store
// such as boltdb.
type keyValueCodec struct {
	// hashes maps keys to the hash of the last content written
	hashes     map[string][]byte
	hashesLock sync.Mutex
}

func newKeyValueCodec() *keyValueCodec {
	return &keyValueCodec{
		hashes: make(map[string][]byte),
	}
}

// hashKey returns a unique key for each hashed boltdb value
func (c *keyValueCodec) hashKey(path string, key []byte) string {
	return path + "-" + string(key)
}

// Put into kv store iff it has changed since the last write. A globally
// unique key is constructed for each value by concatinating the path and key
// passed in.
func (c *keyValueCodec) Put(bkt kvStore, path string, key []byte, val interface{}) error {
	if !bkt.Writable() {
		return fmt.Errorf("bucket must be writable")
	}

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
	hashVal := h.Sum(nil)
	hashKey := c.hashKey(path, key)

	c.hashesLock.Lock()
	persistedHash := c.hashes[hashKey]
	c.hashesLock.Unlock()

	if bytes.Equal(hashVal, persistedHash) {
		return nil
	}

	if err := bkt.Put(key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write data at key %s: %v", key, err)
	}

	// New value written, store hash
	c.hashesLock.Lock()
	c.hashes[hashKey] = hashVal
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
