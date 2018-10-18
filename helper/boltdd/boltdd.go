// boltdd contains a wrapper around BoltDB to deduplicate writes and encode
// values using mgspack.  (dd stands for DeDuplicate)
package boltdd

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"
	"golang.org/x/crypto/blake2b"
)

// DB wraps an underlying bolt.DB to create write deduplicating buckets and
// msgpack encoded values.
type DB struct {
	rootBuckets     map[string]*bucketMeta
	rootBucketsLock sync.Mutex

	bdb *bolt.DB
}

// Open a bolt.DB and wrap it in a write-deduplicating msgpack-encoding
// implementation.
func Open(path string, mode os.FileMode, options *bolt.Options) (*DB, error) {
	bdb, err := bolt.Open(path, mode, options)
	if err != nil {
		return nil, err
	}

	return &DB{
		rootBuckets: make(map[string]*bucketMeta),
		bdb:         bdb,
	}, nil
}

func (db *DB) bucket(btx *bolt.Tx, name []byte) *Bucket {
	bb := btx.Bucket(name)
	if bb == nil {
		return nil
	}

	db.rootBucketsLock.Lock()
	defer db.rootBucketsLock.Unlock()

	b, ok := db.rootBuckets[string(name)]
	if !ok {
		b = newBucketMeta()
		db.rootBuckets[string(name)] = b
	}

	return newBucket(b, bb)
}

func (db *DB) createBucket(btx *bolt.Tx, name []byte) (*Bucket, error) {
	bb, err := btx.CreateBucket(name)
	if err != nil {
		return nil, err
	}

	db.rootBucketsLock.Lock()
	defer db.rootBucketsLock.Unlock()

	// Always create a new Bucket since CreateBucket above fails if the
	// bucket already exists.
	b := newBucketMeta()
	db.rootBuckets[string(name)] = b

	return newBucket(b, bb), nil
}

func (db *DB) createBucketIfNotExists(btx *bolt.Tx, name []byte) (*Bucket, error) {
	bb, err := btx.CreateBucketIfNotExists(name)
	if err != nil {
		return nil, err
	}

	db.rootBucketsLock.Lock()
	defer db.rootBucketsLock.Unlock()

	b, ok := db.rootBuckets[string(name)]
	if !ok {
		b = newBucketMeta()
		db.rootBuckets[string(name)] = b
	}

	return newBucket(b, bb), nil
}

func (db *DB) Update(fn func(*Tx) error) error {
	return db.bdb.Update(func(btx *bolt.Tx) error {
		tx := newTx(db, btx)
		return fn(tx)
	})
}

func (db *DB) View(fn func(*Tx) error) error {
	return db.bdb.View(func(btx *bolt.Tx) error {
		tx := newTx(db, btx)
		return fn(tx)
	})
}

// Close closes the underlying bolt.DB and clears all bucket hashes. DB is
// unusable after closing.
func (db *DB) Close() error {
	db.rootBuckets = nil
	return db.bdb.Close()
}

// BoltDB returns the underlying bolt.DB.
func (db *DB) BoltDB() *bolt.DB {
	return db.bdb
}

type Tx struct {
	db  *DB
	btx *bolt.Tx
}

func newTx(db *DB, btx *bolt.Tx) *Tx {
	return &Tx{
		db:  db,
		btx: btx,
	}
}

// Bucket returns a root bucket or nil if it doesn't exist.
func (tx *Tx) Bucket(name []byte) *Bucket {
	return tx.db.bucket(tx.btx, name)
}

func (tx *Tx) CreateBucket(name []byte) (*Bucket, error) {
	return tx.db.createBucket(tx.btx, name)
}

// CreateBucketIfNotExists returns a root bucket or creates a new one if it
// doesn't already exist.
func (tx *Tx) CreateBucketIfNotExists(name []byte) (*Bucket, error) {
	return tx.db.createBucketIfNotExists(tx.btx, name)
}

// Writable wraps boltdb Tx.Writable.
func (tx *Tx) Writable() bool {
	return tx.btx.Writable()
}

// BoltTx returns the underlying bolt.Tx.
func (tx *Tx) BoltTx() *bolt.Tx {
	return tx.btx
}

// bucketMeta persists metadata -- such as key hashes and child buckets --
// about boltdb Buckets across transactions.
type bucketMeta struct {
	// hashes holds all of the value hashes for keys in this bucket
	hashes     map[string][]byte
	hashesLock sync.Mutex

	// buckets holds all of the child buckets
	buckets     map[string]*bucketMeta
	bucketsLock sync.Mutex
}

func newBucketMeta() *bucketMeta {
	return &bucketMeta{
		hashes:  make(map[string][]byte),
		buckets: make(map[string]*bucketMeta),
	}
}

// getHash of last value written to a key or nil if no hash exists.
func (bm *bucketMeta) getHash(hashKey string) []byte {
	bm.hashesLock.Lock()
	lastHash := bm.hashes[hashKey]
	bm.hashesLock.Unlock()
	return lastHash
}

// setHash of last value written to key.
func (bm *bucketMeta) setHash(hashKey string, hashVal []byte) {
	bm.hashesLock.Lock()
	bm.hashes[hashKey] = hashVal
	bm.hashesLock.Unlock()
}

// delHash deletes a hash value or does nothing if the hash key does not exist.
func (bm *bucketMeta) delHash(hashKey string) {
	bm.hashesLock.Lock()
	delete(bm.hashes, hashKey)
	bm.hashesLock.Unlock()
}

// createBucket metadata entry for the given nested bucket. Overwrites any
// existing entry so caller should ensure bucket does not already exist.
func (bm *bucketMeta) createBucket(name []byte) *bucketMeta {
	bm.bucketsLock.Lock()
	defer bm.bucketsLock.Unlock()

	// Always create a new Bucket since CreateBucket above fails if the
	// bucket already exists.
	b := newBucketMeta()
	bm.buckets[string(name)] = b
	return b
}

// deleteBucket metadata entry for the given nested bucket. Does nothing if
// nested bucket metadata does not exist.
func (bm *bucketMeta) deleteBucket(name []byte) {
	bm.bucketsLock.Lock()
	delete(bm.buckets, string(name))
	bm.bucketsLock.Unlock()

}

// getOrCreateBucket metadata entry for the given nested bucket.
func (bm *bucketMeta) getOrCreateBucket(name []byte) *bucketMeta {
	bm.bucketsLock.Lock()
	defer bm.bucketsLock.Unlock()

	b, ok := bm.buckets[string(name)]
	if !ok {
		b = newBucketMeta()
		bm.buckets[string(name)] = b
	}
	return b
}

type Bucket struct {
	bm         *bucketMeta
	boltBucket *bolt.Bucket
}

// newBucket creates a new view into a bucket backed by a boltdb
// transaction.
func newBucket(b *bucketMeta, bb *bolt.Bucket) *Bucket {
	return &Bucket{
		bm:         b,
		boltBucket: bb,
	}
}

// Put into boltdb iff it has changed since the last write.
func (b *Bucket) Put(key []byte, val interface{}) error {
	// buffer for writing serialized state to
	var buf bytes.Buffer

	// Serialize the object
	if err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(val); err != nil {
		return fmt.Errorf("failed to encode passed object: %v", err)
	}

	// Hash for skipping unnecessary writes
	hashKey := string(key)
	hashVal := blake2b.Sum256(buf.Bytes())

	// lastHash value or nil if it hasn't been hashed yet
	lastHash := b.bm.getHash(hashKey)

	// If the hashes are equal, skip the write
	if bytes.Equal(hashVal[:], lastHash) {
		return nil
	}

	// New value: write it to the underlying boltdb
	if err := b.boltBucket.Put(key, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write data at key %s: %v", key, err)
	}

	// New value written, store hash (bucket path map was created above)
	b.bm.setHash(hashKey, hashVal[:])

	return nil

}

// Get value by key from boltdb or return an error if key not found.
func (b *Bucket) Get(key []byte, obj interface{}) error {
	// Get the raw data from the underlying boltdb
	data := b.boltBucket.Get(key)
	if data == nil {
		return fmt.Errorf("no data at key %v", string(key))
	}

	// Deserialize the object
	if err := codec.NewDecoderBytes(data, structs.MsgpackHandle).Decode(obj); err != nil {
		return fmt.Errorf("failed to decode data into passed object: %v", err)
	}

	return nil
}

// Delete removes a key from the bucket. If the key does not exist then nothing
// is done and a nil error is returned. Returns an error if the bucket was
// created from a read-only transaction.
func (b *Bucket) Delete(key []byte) error {
	err := b.boltBucket.Delete(key)
	b.bm.delHash(string(key))
	return err
}

// Bucket represents a boltdb Bucket and its associated metadata necessary for
// write deduplication. Like bolt.Buckets it is only valid for the duration of
// the transaction that created it.
func (b *Bucket) Bucket(name []byte) *Bucket {
	bb := b.boltBucket.Bucket(name)
	if bb == nil {
		return nil
	}

	bmeta := b.bm.getOrCreateBucket(name)
	return newBucket(bmeta, bb)
}

// CreateBucket creates a new bucket at the given key and returns the new
// bucket. Returns an error if the key already exists, if the bucket name is
// blank, or if the bucket name is too long. The bucket instance is only valid
// for the lifetime of the transaction.
func (b *Bucket) CreateBucket(name []byte) (*Bucket, error) {
	bb, err := b.boltBucket.CreateBucket(name)
	if err != nil {
		return nil, err
	}

	bmeta := b.bm.createBucket(name)
	return newBucket(bmeta, bb), nil
}

// CreateBucketIfNotExists creates a new bucket if it doesn't already exist and
// returns a reference to it. The bucket instance is only valid for the
// lifetime of the transaction.
func (b *Bucket) CreateBucketIfNotExists(name []byte) (*Bucket, error) {
	bb, err := b.boltBucket.CreateBucketIfNotExists(name)
	if err != nil {
		return nil, err
	}

	bmeta := b.bm.getOrCreateBucket(name)
	return newBucket(bmeta, bb), nil
}

// DeleteBucket deletes a child bucket. Returns an error if the bucket does not
// exist or corresponds to a non-bucket key.
func (b *Bucket) DeleteBucket(name []byte) error {
	// Delete the bucket from the underlying boltdb
	err := b.boltBucket.DeleteBucket(name)

	// Remove reference to child bucket
	b.bm.deleteBucket(name)
	return err
}

// BoltBucket returns the internal bolt.Bucket for this Bucket. Only valid
// for the duration of the current transaction.
func (b *Bucket) BoltBucket() *bolt.Bucket {
	return b.boltBucket
}
