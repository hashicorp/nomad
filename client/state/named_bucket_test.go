package state

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/stretchr/testify/require"
)

func setupBoltDB(t *testing.T) (*bolt.DB, func()) {
	dir, err := ioutil.TempDir("", "nomadtest_")
	require.NoError(t, err)
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error removing test dir: %v", err)
		}
	}

	dbFilename := filepath.Join(dir, "nomadtest.db")
	db, err := bolt.Open(dbFilename, 0600, nil)
	if err != nil {
		cleanup()
		t.Fatalf("error creating boltdb: %v", err)
	}

	return db, func() {
		db.Close()
		cleanup()
	}
}

// TestNamedBucket_Path asserts that creating and changing buckets are tracked
// properly by the namedBucket wrapper.
func TestNamedBucket_Path(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	parentBktName, childBktName := []byte("root"), []byte("child")
	parentKey, parentVal := []byte("pkey"), []byte("pval")
	childKey, childVal := []byte("ckey"), []byte("cval")

	require.NoError(db.Update(func(tx *bolt.Tx) error {
		// Trying to open a named bucket from a nonexistent bucket
		// should return nil.
		require.Nil(newNamedBucket(tx, []byte("nonexistent")))

		// Creating a named bucket from a bolt tx should work and set
		// the path and name properly.
		b, err := createNamedBucketIfNotExists(tx, parentBktName)
		require.NoError(err)
		require.Equal(parentBktName, b.Name())
		require.Equal(parentBktName, b.Path())

		// Trying to descend into a nonexistent bucket should return
		// nil.
		require.Nil(b.Bucket([]byte("nonexistent")))

		// Descending into a new bucket should update the path.
		childBkt, err := b.CreateBucket(childBktName)
		require.NoError(err)
		require.Equal(childBktName, childBkt.(*namedBucket).Name())
		require.Equal([]byte("root/child"), childBkt.Path())

		// Assert the parent bucket did not get changed.
		require.Equal(parentBktName, b.Name())
		require.Equal(parentBktName, b.Path())

		// Add entries to both buckets
		require.NoError(b.Put(parentKey, parentVal))
		require.NoError(childBkt.Put(childKey, childVal))
		return nil
	}))

	// Read buckets and values back out
	require.NoError(db.View(func(tx *bolt.Tx) error {
		b := newNamedBucket(tx, parentBktName)
		require.NotNil(b)
		require.Equal(parentVal, b.Get(parentKey))
		require.Nil(b.Get(childKey))

		childBkt := b.Bucket(childBktName)
		require.NotNil(childBkt)
		require.Nil(childBkt.Get(parentKey))
		require.Equal(childVal, childBkt.Get(childKey))
		return nil
	}))
}

// TestNamedBucket_DeleteBucket asserts that deleting a bucket properly purges
// all related keys from the internal hashes map.
func TestNamedBucket_DeleteBucket(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	// Create some nested buckets and keys (key values will just be their names)
	b1Name, c1Name, c2Name, c1c1Name := []byte("b1"), []byte("c1"), []byte("c2"), []byte("c1c1")
	b1k1, c1k1, c2k1, c1c1k1 := []byte("b1k1"), []byte("c1k1"), []byte("c2k1"), []byte("c1c1k1")

	codec := newKeyValueCodec()

	// Create initial db state
	require.NoError(db.Update(func(tx *bolt.Tx) error {
		// Create bucket 1 and key
		b1, err := createNamedBucketIfNotExists(tx, b1Name)
		require.NoError(err)
		require.NoError(codec.Put(b1, b1k1, b1k1))

		// Create child bucket 1 and key
		c1, err := b1.CreateBucketIfNotExists(c1Name)
		require.NoError(err)
		require.NoError(codec.Put(c1, c1k1, c1k1))

		// Create child-child bucket 1 and key
		c1c1, err := c1.CreateBucketIfNotExists(c1c1Name)
		require.NoError(err)
		require.NoError(codec.Put(c1c1, c1c1k1, c1c1k1))

		// Create child bucket 2 and key
		c2, err := b1.CreateBucketIfNotExists(c2Name)
		require.NoError(err)
		require.NoError(codec.Put(c2, c2k1, c2k1))
		return nil
	}))

	// codec should be tracking 4 hash buckets (b1, c1, c2, c1c1)
	require.Len(codec.hashes, 4)

	// Delete c1
	require.NoError(db.Update(func(tx *bolt.Tx) error {
		b1 := newNamedBucket(tx, b1Name)
		return codec.DeleteBucket(b1, c1Name)
	}))

	START HERE // We don't appear to be properly deleting the sub-bucket
	// codec should be tracking 2 hash buckets (b1, c2)
	require.Len(codec.hashes, 2)

	// Assert all of c1 is gone
	require.NoError(db.View(func(tx *bolt.Tx) error {
		return nil
	}))
}
