package boltdd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

type testingT interface {
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}

func setupBoltDB(t testingT) (*DB, func()) {
	dir, err := ioutil.TempDir("", "nomadtest_")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}

	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error removing test dir: %v", err)
		}
	}

	dbFilename := filepath.Join(dir, "nomadtest.db")
	db, err := Open(dbFilename, 0600, nil)
	if err != nil {
		cleanup()
		t.Fatalf("error creating boltdb: %v", err)
	}

	return db, func() {
		db.Close()
		cleanup()
	}
}

func TestDB_Open(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	require.Equal(0, db.BoltDB().Stats().TxStats.Write)
}

func TestDB_Close(t *testing.T) {
	t.Parallel()

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	db.Close()

	require.Equal(t, db.Update(func(tx *Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("foo"))
		return err
	}), bolt.ErrDatabaseNotOpen)

	require.Equal(t, db.Update(func(tx *Tx) error {
		_, err := tx.CreateBucket([]byte("foo"))
		return err
	}), bolt.ErrDatabaseNotOpen)
}

func TestBucket_Create(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	name := []byte("create_test")

	require.NoError(db.Update(func(tx *Tx) error {
		// Trying to get a nonexistent bucket should return nil
		require.Nil(tx.Bucket(name))

		// Creating a nonexistent bucket should work
		b, err := tx.CreateBucket(name)
		require.NoError(err)
		require.NotNil(b)

		// Recreating a bucket that exists should fail
		b, err = tx.CreateBucket(name)
		require.Error(err)
		require.Nil(b)

		// get or create should work
		b, err = tx.CreateBucketIfNotExists(name)
		require.NoError(err)
		require.NotNil(b)
		return nil
	}))

	// Bucket should be visible
	require.NoError(db.View(func(tx *Tx) error {
		require.NotNil(tx.Bucket(name))
		return nil
	}))
}

func TestBucket_DedupeWrites(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	bname := []byte("dedupewrites_test")
	k1name := []byte("k1")
	k2name := []byte("k2")

	// Put 2 keys
	require.NoError(db.Update(func(tx *Tx) error {
		b, err := tx.CreateBucket(bname)
		require.NoError(err)

		require.NoError(b.Put(k1name, k1name))
		require.NoError(b.Put(k2name, k2name))
		return nil
	}))

	// Assert there was at least 1 write
	origWrites := db.BoltDB().Stats().TxStats.Write
	require.NotZero(origWrites)

	// Write the same values again and expect no new writes
	require.NoError(db.Update(func(tx *Tx) error {
		b := tx.Bucket(bname)
		require.NoError(b.Put(k1name, k1name))
		require.NoError(b.Put(k2name, k2name))
		return nil
	}))

	putWrites := db.BoltDB().Stats().TxStats.Write

	// Unforunately every committed transaction causes two writes, so this
	// only saves 1 write operation
	require.Equal(origWrites+2, putWrites)

	// Write new values and assert more writes took place
	require.NoError(db.Update(func(tx *Tx) error {
		b := tx.Bucket(bname)
		require.NoError(b.Put(k1name, []byte("newval1")))
		require.NoError(b.Put(k2name, []byte("newval2")))
		return nil
	}))

	putWrites2 := db.BoltDB().Stats().TxStats.Write

	// Expect 3 additional writes: 2 for the transaction and one for the
	// dirty page
	require.Equal(putWrites+3, putWrites2)
}

func TestBucket_Delete(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	parentName := []byte("delete_test")
	parentKey := []byte("parent_key")
	childName := []byte("child")
	childKey := []byte("child_key")
	grandchildName1 := []byte("grandchild1")
	grandchildKey1 := []byte("grandchild_key1")
	grandchildName2 := []byte("grandchild2")
	grandchildKey2 := []byte("grandchild_key2")

	// Create a parent bucket with 1 child and 2 grandchildren
	require.NoError(db.Update(func(tx *Tx) error {
		pb, err := tx.CreateBucket(parentName)
		require.NoError(err)

		require.NoError(pb.Put(parentKey, parentKey))

		child, err := pb.CreateBucket(childName)
		require.NoError(err)

		require.NoError(child.Put(childKey, childKey))

		grandchild1, err := child.CreateBucket(grandchildName1)
		require.NoError(err)

		require.NoError(grandchild1.Put(grandchildKey1, grandchildKey1))

		grandchild2, err := child.CreateBucket(grandchildName2)
		require.NoError(err)

		require.NoError(grandchild2.Put(grandchildKey2, grandchildKey2))
		return nil
	}))

	// Verify grandchild keys wrote
	require.NoError(db.View(func(tx *Tx) error {
		grandchild1 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName1)
		var v1 []byte
		grandchild1.Get(grandchildKey1, &v1)
		require.Equal(grandchildKey1, v1)

		grandchild2 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName2)
		var v2 []byte
		grandchild2.Get(grandchildKey2, &v2)
		require.Equal(grandchildKey2, v2)
		return nil
	}))

	// Delete grandchildKey1 and grandchild2
	require.NoError(db.Update(func(tx *Tx) error {
		child := tx.Bucket(parentName).Bucket(childName)

		require.NoError(child.DeleteBucket(grandchildName2))

		grandchild1 := child.Bucket(grandchildName1)
		require.NoError(grandchild1.Delete(grandchildKey1))
		return nil
	}))

	// Ensure grandchild2 alone was deleted
	require.NoError(db.View(func(tx *Tx) error {
		grandchild1 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName1)
		var v1 []byte
		grandchild1.Get(grandchildKey1, &v1)
		require.Equal(([]byte)(nil), v1)

		grandchild2 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName2)
		require.Nil(grandchild2)
		return nil
	}))

	// Deleting child bucket should delete grandchild1 as well
	require.NoError(db.Update(func(tx *Tx) error {
		parent := tx.Bucket(parentName)
		require.NoError(parent.DeleteBucket(childName))

		// Recreate child bucket and ensure childKey and grandchild are gone
		child, err := parent.CreateBucket(childName)
		require.NoError(err)

		var v []byte
		err = child.Get(childKey, &v)
		require.Error(err)
		require.True(IsErrNotFound(err))
		require.Equal(([]byte)(nil), v)

		require.Nil(child.Bucket(grandchildName1))

		// Rewrite childKey1 to make sure it doesn't get dedupe incorrectly
		require.NoError(child.Put(childKey, childKey))
		return nil
	}))

	// Ensure childKey1 was rewritten and not deduped incorrectly
	require.NoError(db.View(func(tx *Tx) error {
		var v []byte
		require.NoError(tx.Bucket(parentName).Bucket(childName).Get(childKey, &v))
		require.Equal(childKey, v)
		return nil
	}))
}

func BenchmarkWriteDeduplication_On(b *testing.B) {
	db, cleanup := setupBoltDB(b)
	defer cleanup()

	bucketName := []byte("allocations")
	alloc := mock.Alloc()
	allocID := []byte(alloc.ID)

	err := db.Update(func(tx *Tx) error {
		allocs, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		return allocs.Put(allocID, alloc)
	})

	if err != nil {
		b.Fatalf("error setting up: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.Update(func(tx *Tx) error {
			return tx.Bucket(bucketName).Put(allocID, alloc)
		})

		if err != nil {
			b.Fatalf("error at runtime: %v", err)
		}
	}
}

func BenchmarkWriteDeduplication_Off(b *testing.B) {
	dir, err := ioutil.TempDir("", "nomadtest_")
	if err != nil {
		b.Fatalf("error creating tempdir: %v", err)
	}

	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			b.Logf("error removing test dir: %v", err)
		}
	}()

	dbFilename := filepath.Join(dir, "nomadtest.db")
	db, err := Open(dbFilename, 0600, nil)
	if err != nil {
		b.Fatalf("error creating boltdb: %v", err)
	}

	defer db.Close()

	bucketName := []byte("allocations")
	alloc := mock.Alloc()
	allocID := []byte(alloc.ID)

	err = db.Update(func(tx *Tx) error {
		allocs, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		if err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(alloc); err != nil {
			return fmt.Errorf("failed to encode passed object: %v", err)
		}

		return allocs.Put(allocID, buf)
	})

	if err != nil {
		b.Fatalf("error setting up: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.Update(func(tx *Tx) error {
			var buf bytes.Buffer
			if err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(alloc); err != nil {
				return fmt.Errorf("failed to encode passed object: %v", err)
			}

			return tx.Bucket(bucketName).Put(allocID, buf)
		})

		if err != nil {
			b.Fatalf("error at runtime: %v", err)
		}
	}
}
