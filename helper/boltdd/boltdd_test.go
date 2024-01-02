// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package boltdd

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"go.etcd.io/bbolt"
)

const (
	testDB      = "nomad-test.db"
	testDBPerms = 0600
)

// a simple struct type for testing msg pack en/decoding
type employee struct {
	Name string
	ID   int
}

func setupBoltDB(t testing.TB) *DB {
	dir := t.TempDir()

	dbFilename := filepath.Join(dir, testDB)
	db, err := Open(dbFilename, testDBPerms, nil)
	must.NoError(t, err)

	t.Cleanup(func() {
		must.NoError(t, db.Close())
	})

	return db
}

func TestDB_Open(t *testing.T) {
	ci.Parallel(t)
	db := setupBoltDB(t)
	must.Zero(t, db.BoltDB().Stats().TxStats.Write)
}

func TestDB_Close(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	must.NoError(t, db.Close())

	must.Eq(t, db.Update(func(tx *Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("foo"))
		return err
	}), bbolt.ErrDatabaseNotOpen)

	must.Eq(t, db.Update(func(tx *Tx) error {
		_, err := tx.CreateBucket([]byte("foo"))
		return err
	}), bbolt.ErrDatabaseNotOpen)
}

func TestBucket_Create(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	name := []byte("create_test")

	must.NoError(t, db.Update(func(tx *Tx) error {
		// Trying to get a nonexistent bucket should return nil
		must.Nil(t, tx.Bucket(name))

		// Creating a nonexistent bucket should work
		b, err := tx.CreateBucket(name)
		must.NoError(t, err)
		must.NotNil(t, b)

		// Recreating a bucket that exists should fail
		b, err = tx.CreateBucket(name)
		must.Error(t, err)
		must.Nil(t, b)

		// get or create should work
		b, err = tx.CreateBucketIfNotExists(name)
		must.NoError(t, err)
		must.NotNil(t, b)
		return nil
	}))

	// Bucket should be visible
	must.NoError(t, db.View(func(tx *Tx) error {
		must.NotNil(t, tx.Bucket(name))
		return nil
	}))
}

func TestBucket_Iterate(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	bucket := []byte("iterate_test")

	must.NoError(t, db.Update(func(tx *Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		must.NoError(t, err)
		must.NotNil(t, b)

		must.NoError(t, b.Put([]byte("ceo"), employee{Name: "dave", ID: 15}))
		must.NoError(t, b.Put([]byte("founder"), employee{Name: "mitchell", ID: 1}))
		must.NoError(t, b.Put([]byte("cto"), employee{Name: "armon", ID: 2}))
		return nil
	}))

	t.Run("success", func(t *testing.T) {
		var result []employee
		err := db.View(func(tx *Tx) error {
			b := tx.Bucket(bucket)
			return Iterate(b, nil, func(key []byte, e employee) {
				result = append(result, e)
			})
		})
		must.NoError(t, err)
		must.Eq(t, []employee{
			{"dave", 15}, {"armon", 2}, {"mitchell", 1},
		}, result)
	})

	t.Run("failure", func(t *testing.T) {
		err := db.View(func(tx *Tx) error {
			b := tx.Bucket(bucket)
			// will fail to encode employee into an int
			return Iterate(b, nil, func(key []byte, i int) {
				must.Unreachable(t)
			})
		})
		must.Error(t, err)
	})
}

func TestBucket_DeletePrefix(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	bucket := []byte("delete_prefix_test")

	must.NoError(t, db.Update(func(tx *Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucket)
		must.NoError(t, err)
		must.NotNil(t, b)

		must.NoError(t, b.Put([]byte("exec_a"), employee{Name: "dave", ID: 15}))
		must.NoError(t, b.Put([]byte("intern_a"), employee{Name: "alice", ID: 7384}))
		must.NoError(t, b.Put([]byte("exec_c"), employee{Name: "armon", ID: 2}))
		must.NoError(t, b.Put([]byte("intern_b"), employee{Name: "bob", ID: 7312}))
		must.NoError(t, b.Put([]byte("exec_b"), employee{Name: "mitchell", ID: 1}))
		return nil
	}))

	// remove interns
	must.NoError(t, db.Update(func(tx *Tx) error {
		bkt := tx.Bucket(bucket)
		return bkt.DeletePrefix([]byte("intern_"))
	}))

	// assert 3 exec remain
	var result []employee
	err := db.View(func(tx *Tx) error {
		bkt := tx.Bucket(bucket)
		return Iterate(bkt, nil, func(k []byte, e employee) {
			result = append(result, e)
		})
	})
	must.NoError(t, err)
	must.Eq(t, []employee{
		{"dave", 15}, {"mitchell", 1}, {"armon", 2},
	}, result)
}

func TestBucket_DedupeWrites(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	bname := []byte("dedupewrites_test")
	k1name := []byte("k1")
	k2name := []byte("k2")

	// Put 2 keys
	must.NoError(t, db.Update(func(tx *Tx) error {
		b, err := tx.CreateBucket(bname)
		must.NoError(t, err)
		must.NoError(t, b.Put(k1name, k1name))
		must.NoError(t, b.Put(k2name, k2name))
		return nil
	}))

	// Assert there was at least 1 write
	origWrites := db.BoltDB().Stats().TxStats.Write
	must.Positive(t, origWrites)

	// Write the same values again and expect no new writes
	must.NoError(t, db.Update(func(tx *Tx) error {
		b := tx.Bucket(bname)
		must.NoError(t, b.Put(k1name, k1name))
		must.NoError(t, b.Put(k2name, k2name))
		return nil
	}))

	putWrites := db.BoltDB().Stats().TxStats.Write

	// Unfortunately every committed transaction causes two writes, so this
	// only saves 1 write operation
	must.Eq(t, origWrites+2, putWrites)

	// Write new values and assert more writes took place
	must.NoError(t, db.Update(func(tx *Tx) error {
		b := tx.Bucket(bname)
		must.NoError(t, b.Put(k1name, []byte("newval1")))
		must.NoError(t, b.Put(k2name, []byte("newval2")))
		return nil
	}))

	putWrites2 := db.BoltDB().Stats().TxStats.Write

	// Expect 3 additional writes: 2 for the transaction and one for the
	// dirty page
	must.Eq(t, putWrites+3, putWrites2)
}

func TestBucket_Delete(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	parentName := []byte("delete_test")
	parentKey := []byte("parent_key")
	childName := []byte("child")
	childKey := []byte("child_key")
	grandchildName1 := []byte("grandchild1")
	grandchildKey1 := []byte("grandchild_key1")
	grandchildName2 := []byte("grandchild2")
	grandchildKey2 := []byte("grandchild_key2")

	// Create a parent bucket with 1 child and 2 grandchildren
	must.NoError(t, db.Update(func(tx *Tx) error {
		pb, err := tx.CreateBucket(parentName)
		must.NoError(t, err)

		must.NoError(t, pb.Put(parentKey, parentKey))

		child, err := pb.CreateBucket(childName)
		must.NoError(t, err)

		must.NoError(t, child.Put(childKey, childKey))

		grandchild1, err := child.CreateBucket(grandchildName1)
		must.NoError(t, err)

		must.NoError(t, grandchild1.Put(grandchildKey1, grandchildKey1))

		grandchild2, err := child.CreateBucket(grandchildName2)
		must.NoError(t, err)

		must.NoError(t, grandchild2.Put(grandchildKey2, grandchildKey2))
		return nil
	}))

	// Verify grandchild keys wrote
	must.NoError(t, db.View(func(tx *Tx) error {
		grandchild1 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName1)
		var v1 []byte
		must.NoError(t, grandchild1.Get(grandchildKey1, &v1))
		must.Eq(t, grandchildKey1, v1)

		grandchild2 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName2)
		var v2 []byte
		must.NoError(t, grandchild2.Get(grandchildKey2, &v2))
		must.Eq(t, grandchildKey2, v2)
		return nil
	}))

	// Delete grandchildKey1 and grandchild2
	must.NoError(t, db.Update(func(tx *Tx) error {
		child := tx.Bucket(parentName).Bucket(childName)
		must.NoError(t, child.DeleteBucket(grandchildName2))

		grandchild1 := child.Bucket(grandchildName1)
		must.NoError(t, grandchild1.Delete(grandchildKey1))
		return nil
	}))

	// Ensure grandchild2 alone was deleted
	must.NoError(t, db.View(func(tx *Tx) error {
		grandchild1 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName1)
		var v1 []byte
		must.Error(t, grandchild1.Get(grandchildKey1, &v1))
		must.Eq(t, ([]byte)(nil), v1)

		grandchild2 := tx.Bucket(parentName).Bucket(childName).Bucket(grandchildName2)
		must.Nil(t, grandchild2)
		return nil
	}))

	// Deleting child bucket should delete grandchild1 as well
	must.NoError(t, db.Update(func(tx *Tx) error {
		parent := tx.Bucket(parentName)
		must.NoError(t, parent.DeleteBucket(childName))

		// Recreate child bucket and ensure childKey and grandchild are gone
		child, err := parent.CreateBucket(childName)
		must.NoError(t, err)

		var v []byte
		err = child.Get(childKey, &v)
		must.Error(t, err)
		must.True(t, IsErrNotFound(err))
		must.Eq(t, ([]byte)(nil), v)

		must.Nil(t, child.Bucket(grandchildName1))

		// Rewrite childKey1 to make sure it doesn't get de-dupe incorrectly
		must.NoError(t, child.Put(childKey, childKey))
		return nil
	}))

	// Ensure childKey1 was rewritten and not de-duped incorrectly
	must.NoError(t, db.View(func(tx *Tx) error {
		var v []byte
		must.NoError(t, tx.Bucket(parentName).Bucket(childName).Get(childKey, &v))
		must.Eq(t, childKey, v)
		return nil
	}))
}

func BenchmarkWriteDeduplication_On(b *testing.B) {
	db := setupBoltDB(b)

	bucketName := []byte("allocations")
	alloc := mock.Alloc()
	allocID := []byte(alloc.ID)

	must.NoError(b, db.Update(func(tx *Tx) error {
		allocs, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		return allocs.Put(allocID, alloc)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		must.NoError(b, db.Update(func(tx *Tx) error {
			return tx.Bucket(bucketName).Put(allocID, alloc)
		}))
	}
}

func BenchmarkWriteDeduplication_Off(b *testing.B) {
	dir := b.TempDir()

	dbFilename := filepath.Join(dir, testDB)
	db, openErr := Open(dbFilename, testDBPerms, nil)
	must.NoError(b, openErr)

	b.Cleanup(func() {
		must.NoError(b, db.Close())
	})

	bucketName := []byte("allocations")
	alloc := mock.Alloc()
	allocID := []byte(alloc.ID)

	must.NoError(b, db.Update(func(tx *Tx) error {
		allocs, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		if err = codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(alloc); err != nil {
			return fmt.Errorf("failed to encode passed object: %v", err)
		}

		return allocs.Put(allocID, buf)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		must.NoError(b, db.Update(func(tx *Tx) error {
			var buf bytes.Buffer
			if err := codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(alloc); err != nil {
				return fmt.Errorf("failed to encode passed object: %v", err)
			}
			return tx.Bucket(bucketName).Put(allocID, buf)
		}))
	}
}
