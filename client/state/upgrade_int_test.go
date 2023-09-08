// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/config"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/devicemanager"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/lib/proclib"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	. "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper/boltdd"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// TestBoltStateDB_Upgrade_Ok asserts upgading an old state db does not error
// during upgrade and restore.
func TestBoltStateDB_UpgradeOld_Ok(t *testing.T) {
	ci.Parallel(t)

	dbFromTestFile := func(t *testing.T, dir, fn string) *BoltStateDB {

		var src io.ReadCloser
		src, err := os.Open(fn)
		require.NoError(t, err)
		defer src.Close()

		// testdata may be gzip'd; decode on copy
		if strings.HasSuffix(fn, ".gz") {
			src, err = gzip.NewReader(src)
			require.NoError(t, err)
		}

		dst, err := os.Create(filepath.Join(dir, "state.db"))
		require.NoError(t, err)

		// Copy test files before testing them for safety
		_, err = io.Copy(dst, src)
		require.NoError(t, err)

		require.NoError(t, src.Close())

		dbI, err := NewBoltStateDB(testlog.HCLogger(t), dir)
		require.NoError(t, err)

		db := dbI.(*BoltStateDB)
		return db
	}

	pre09files := []string{
		"testdata/state-0.7.1.db.gz",
		"testdata/state-0.8.6-empty.db.gz",
		"testdata/state-0.8.6-no-deploy.db.gz"}

	for _, fn := range pre09files {
		t.Run(fn, func(t *testing.T) {

			dir := t.TempDir()

			db := dbFromTestFile(t, dir, fn)
			defer db.Close()

			// Simply opening old files should *not* alter them
			require.NoError(t, db.DB().View(func(tx *boltdd.Tx) error {
				b := tx.Bucket([]byte("meta"))
				if b != nil {
					return fmt.Errorf("meta bucket found but should not exist yet!")
				}
				return nil
			}))

			to09, to12, err := NeedsUpgrade(db.DB().BoltDB())
			require.NoError(t, err)
			require.True(t, to09)
			require.True(t, to12)

			// Attempt the upgrade
			require.NoError(t, db.Upgrade())

			to09, to12, err = NeedsUpgrade(db.DB().BoltDB())
			require.NoError(t, err)
			require.False(t, to09)
			require.False(t, to12)

			// Ensure Allocations can be restored and
			// NewAR/AR.Restore do not error.
			allocs, errs, err := db.GetAllAllocations()
			require.NoError(t, err)
			assert.Len(t, errs, 0)

			for _, alloc := range allocs {
				checkUpgradedAlloc(t, dir, db, alloc)
			}

			// Should be nil for all upgrades
			ps, err := db.GetDevicePluginState()
			require.NoError(t, err)
			require.Nil(t, ps)

			ps = &dmstate.PluginState{
				ReattachConfigs: map[string]*pstructs.ReattachConfig{
					"test": {Pid: 1},
				},
			}
			require.NoError(t, db.PutDevicePluginState(ps))

			registry, err := db.GetDynamicPluginRegistryState()
			require.Nil(t, registry)

			require.NoError(t, err)
			require.NoError(t, db.Close())
		})
	}

	t.Run("testdata/state-1.2.6.db.gz", func(t *testing.T) {
		fn := "testdata/state-1.2.6.db.gz"
		dir := t.TempDir()

		db := dbFromTestFile(t, dir, fn)
		defer db.Close()

		// Simply opening old files should *not* alter them
		db.DB().BoltDB().View(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte("meta"))
			if b == nil {
				return fmt.Errorf("meta bucket should exist")
			}
			v := b.Get([]byte("version"))
			if len(v) == 0 {
				return fmt.Errorf("meta version is missing")
			}
			if !bytes.Equal(v, []byte{'2'}) {
				return fmt.Errorf("meta version should not have changed")
			}
			return nil
		})

		to09, to12, err := NeedsUpgrade(db.DB().BoltDB())
		require.NoError(t, err)
		require.False(t, to09)
		require.True(t, to12)

		// Attempt the upgrade
		require.NoError(t, db.Upgrade())

		to09, to12, err = NeedsUpgrade(db.DB().BoltDB())
		require.NoError(t, err)
		require.False(t, to09)
		require.False(t, to12)

		registry, err := db.GetDynamicPluginRegistryState()
		require.NoError(t, err)
		require.NotNil(t, registry)
		require.Len(t, registry.Plugins["csi-node"], 2)

		require.NoError(t, db.Close())
	})
}

// checkUpgradedAlloc creates and restores an AllocRunner from an upgraded
// database.
//
// It does not call AR.Run as its intended to be used against a wide test
// corpus in testdata that may be expensive to run and require unavailable
// dependencies.
func checkUpgradedAlloc(t *testing.T, path string, db StateDB, alloc *structs.Allocation) {
	_, err := db.GetDeploymentStatus(alloc.ID)
	assert.NoError(t, err)

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, task := range tg.Tasks {
		_, _, err := db.GetTaskRunnerState(alloc.ID, task.Name)
		require.NoError(t, err)
	}

	clientConf, cleanup := clientconfig.TestClientConfig(t)

	// Does *not* cleanup overridden StateDir below. That's left alone for
	// the caller to cleanup.
	defer cleanup()

	clientConf.StateDir = path

	conf := &config.AllocRunnerConfig{
		Alloc:             alloc,
		Logger:            clientConf.Logger,
		ClientConfig:      clientConf,
		StateDB:           db,
		Consul:            regMock.NewServiceRegistrationHandler(clientConf.Logger),
		Vault:             vaultclient.NewMockVaultClient(),
		StateUpdater:      &allocrunner.MockStateUpdater{},
		PrevAllocWatcher:  allocwatcher.NoopPrevAlloc{},
		PrevAllocMigrator: allocwatcher.NoopPrevAlloc{},
		DeviceManager:     devicemanager.NoopMockManager(),
		DriverManager:     drivermanager.TestDriverManager(t),
		Wranglers:         proclib.New(&proclib.Configs{Logger: testlog.HCLogger(t)}),
	}
	ar, err := allocrunner.NewAllocRunner(conf)
	require.NoError(t, err)

	// AllocRunner.Restore should not error
	require.NoError(t, ar.Restore())
}
