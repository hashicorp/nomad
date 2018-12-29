package state

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/nomad/helper/boltdd"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

func setupBoltDB(t *testing.T) (*bolt.DB, func()) {
	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)

	db, err := bolt.Open(filepath.Join(dir, "state.db"), 0666, nil)
	if err != nil {
		os.RemoveAll(dir)
		require.NoError(t, err)
	}

	return db, func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}
}

// TestUpgrade_NeedsUpgrade_New asserts new state dbs do not need upgrading.
func TestUpgrade_NeedsUpgrade_New(t *testing.T) {
	t.Parallel()

	// Setting up a new StateDB should initialize it at the latest version.
	db, cleanup := setupBoltStateDB(t)
	defer cleanup()

	up, err := NeedsUpgrade(db.DB().BoltDB())
	require.NoError(t, err)
	require.False(t, up)
}

// TestUpgrade_NeedsUpgrade_Old asserts state dbs with just the alloctions
// bucket *do* need upgrading.
func TestUpgrade_NeedsUpgrade_Old(t *testing.T) {
	t.Parallel()

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	// Create the allocations bucket which exists in both the old and 0.9
	// schemas
	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(allocationsBucketName)
		return err
	}))

	up, err := NeedsUpgrade(db)
	require.NoError(t, err)
	require.True(t, up)

	// Adding meta should mark it as upgraded
	require.NoError(t, db.Update(addMeta))

	up, err = NeedsUpgrade(db)
	require.NoError(t, err)
	require.False(t, up)
}

// TestUpgrade_NeedsUpgrade_Error asserts that an error is returned from
// NeedsUpgrade if an invalid db version is found. This is a safety measure to
// prevent invalid and unintentional upgrades when downgrading Nomad.
func TestUpgrade_NeedsUpgrade_Error(t *testing.T) {
	t.Parallel()

	cases := [][]byte{
		{'"', '2', '"'}, // wrong type
		{'1'},           // wrong version (never existed)
		{'3'},           // wrong version (future)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			db, cleanup := setupBoltDB(t)
			defer cleanup()

			require.NoError(t, db.Update(func(tx *bolt.Tx) error {
				bkt, err := tx.CreateBucketIfNotExists(metaBucketName)
				require.NoError(t, err)

				return bkt.Put(metaVersionKey, tc)
			}))

			_, err := NeedsUpgrade(db)
			require.Error(t, err)
		})
	}
}

// TestUpgrade_DeleteInvalidAllocs asserts invalid allocations are deleted
// during state upgades instead of failing the entire agent.
func TestUpgrade_DeleteInvalidAllocs_NoAlloc(t *testing.T) {
	t.Parallel()

	bdb, cleanup := setupBoltDB(t)
	defer cleanup()

	db := boltdd.New(bdb)

	allocID := []byte(uuid.Generate())

	// Create an allocation bucket with no `alloc` key. This is an observed
	// pre-0.9 state corruption that should result in the allocation being
	// dropped while allowing the upgrade to continue.
	require.NoError(t, db.Update(func(tx *boltdd.Tx) error {
		parentBkt, err := tx.CreateBucket(allocationsBucketName)
		if err != nil {
			return err
		}

		_, err = parentBkt.CreateBucket(allocID)
		return err
	}))

	// Perform the Upgrade
	require.NoError(t, db.Update(func(tx *boltdd.Tx) error {
		return UpgradeAllocs(testlog.HCLogger(t), tx)
	}))

	// Assert invalid allocation bucket was removed
	require.NoError(t, db.View(func(tx *boltdd.Tx) error {
		parentBkt := tx.Bucket(allocationsBucketName)
		if parentBkt == nil {
			return fmt.Errorf("parent allocations bucket should not have been removed")
		}

		if parentBkt.Bucket(allocID) != nil {
			return fmt.Errorf("invalid alloc bucket should have been deleted")
		}

		return nil
	}))
}

// TestUpgrade_DeleteInvalidTaskEntries asserts invalid entries under a task
// bucket are deleted.
func TestUpgrade_upgradeTaskBucket_InvalidEntries(t *testing.T) {
	t.Parallel()

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	taskName := []byte("fake-task")

	// Insert unexpected bucket, unexpected key, and missing simple-all
	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucket(taskName)
		if err != nil {
			return err
		}

		_, err = bkt.CreateBucket([]byte("unexpectedBucket"))
		if err != nil {
			return err
		}

		return bkt.Put([]byte("unexepectedKey"), []byte{'x'})
	}))

	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(taskName)

		// upgradeTaskBucket should fail
		state, err := upgradeTaskBucket(testlog.HCLogger(t), bkt)
		require.Nil(t, state)
		require.Error(t, err)

		// Invalid entries should have been deleted
		cur := bkt.Cursor()
		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			t.Errorf("unexpected entry found: key=%q value=%q", k, v)
		}

		return nil
	}))
}
