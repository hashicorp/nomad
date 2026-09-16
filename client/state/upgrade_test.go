// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func setupBoltDB(t *testing.T) *bbolt.DB {
	dir := t.TempDir()

	db, err := bbolt.Open(filepath.Join(dir, "state.db"), 0666, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}

// TestUpgrade_NeedsUpgrade_New asserts new state dbs do not need upgrading.
func TestUpgrade_NeedsUpgrade_New(t *testing.T) {
	ci.Parallel(t)

	// Setting up a new StateDB should initialize it at the latest version.
	db := setupBoltStateDB(t)

	to12, err := NeedsUpgrade(db.DB().BoltDB())
	must.NoError(t, err)
	must.False(t, to12)
}

// TestUpgrade_NeedsUpgrade_Old asserts state dbs with just the alloctions
// bucket *do* need upgrading.
func TestUpgrade_NeedsUpgrade_Old(t *testing.T) {
	ci.Parallel(t)

	db := setupBoltDB(t)

	// Create the allocations bucket which exists in both the old and 0.9
	// schemas
	must.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(allocationsBucketName)
		return err
	}))

	to12, err := NeedsUpgrade(db)
	must.NoError(t, err)
	must.True(t, to12)

	// Adding meta should mark it as upgraded
	must.NoError(t, db.Update(addMeta))

	to12, err = NeedsUpgrade(db)
	must.NoError(t, err)
	must.False(t, to12)
}

// TestUpgrade_NeedsUpgrade_Error asserts that an error is returned from
// NeedsUpgrade if an invalid db version is found. This is a safety measure to
// prevent invalid and unintentional upgrades when downgrading Nomad.
func TestUpgrade_NeedsUpgrade_Error(t *testing.T) {
	ci.Parallel(t)

	cases := [][]byte{
		{'"', '2', '"'}, // wrong type
		{'1'},           // wrong version (never existed)
		{'4'},           // wrong version (future)
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			db := setupBoltDB(t)

			require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
				bkt, err := tx.CreateBucketIfNotExists(metaBucketName)
				require.NoError(t, err)

				return bkt.Put(metaVersionKey, tc)
			}))

			_, err := NeedsUpgrade(db)
			require.Error(t, err)
		})
	}
}
