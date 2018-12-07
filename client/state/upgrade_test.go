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

// TestUpgrade_NeedsUpgrade_New asserts new state dbs do not need upgrading.
func TestUpgrade_NeedsUpgrade_New(t *testing.T) {
	t.Parallel()

	db, cleanup := setupBoltDB(t)
	defer cleanup()

	up, err := NeedsUpgrade(db.DB().BoltDB())
	require.NoError(t, err)
	require.False(t, up)
}

// TestUpgrade_NeedsUpgrade_Old asserts state dbs with just the alloctions
// bucket *do* need upgrading.
func TestUpgrade_NeedsUpgrade_Old(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	db, err := bolt.Open(filepath.Join(dir, "state.db"), 0666, nil)
	require.NoError(t, err)
	defer db.Close()

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

// TestUpgrade_DeleteInvalidAllocs asserts invalid allocations are deleted
// during state upgades instead of failing the entire agent.
func TestUpgrade_DeleteInvalidAllocs_NoAlloc(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	db, err := boltdd.Open(filepath.Join(dir, "state.db"), 0666, nil)
	require.NoError(t, err)
	defer db.Close()

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
