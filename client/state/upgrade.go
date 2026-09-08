// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"bytes"
	"container/list"
	"fmt"
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper/boltdd"
	"go.etcd.io/bbolt"
)

// NeedsUpgrade returns true if the BoltDB needs upgrading or false if it is
// already up to date.
func NeedsUpgrade(bdb *bbolt.DB) (upgradeTo13 bool, err error) {
	upgradeTo13 = true
	err = bdb.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(metaBucketName)
		if b == nil {
			// No meta bucket; upgrade
			return nil
		}

		v := b.Get(metaVersionKey)
		if len(v) == 0 {
			// No version; upgrade
			return nil
		}

		if bytes.Equal(v, []byte{'2'}) {
			return nil
		}

		if bytes.Equal(v, metaVersion) {
			upgradeTo13 = false
			return nil
		}

		// Version exists but does not match. Abort.
		return fmt.Errorf("incompatible state version. expected %q but found %q",
			metaVersion, v)

	})

	return
}

// addMeta adds version metadata to BoltDB to mark it as upgraded and
// should be run at the end of the upgrade transaction.
func addMeta(tx *bbolt.Tx) error {
	// Create the meta bucket if it doesn't exist
	bkt, err := tx.CreateBucketIfNotExists(metaBucketName)
	if err != nil {
		return err
	}
	return bkt.Put(metaVersionKey, metaVersion)
}

// backupDB backs up the existing state database prior to upgrade overwriting
// previous backups.
func backupDB(bdb *bbolt.DB, dst string) error {
	fd, err := os.Create(dst)
	if err != nil {
		return err
	}

	return bdb.View(func(tx *bbolt.Tx) error {
		if _, err := tx.WriteTo(fd); err != nil {
			fd.Close()
			return err
		}

		return fd.Close()
	})
}

func UpgradeDynamicPluginRegistry(logger hclog.Logger, tx *boltdd.Tx) error {

	dynamicBkt := tx.Bucket(dynamicPluginBucketName)
	if dynamicBkt == nil {
		return nil // no previous plugins upgrade
	}

	oldState := &RegistryState12{}
	if err := dynamicBkt.Get(registryStateKey, oldState); err != nil {
		if !boltdd.IsErrNotFound(err) {
			return fmt.Errorf("failed to read dynamic plugin registry state: %v", err)
		}
	}

	newState := &dynamicplugins.RegistryState{
		Plugins: make(map[string]map[string]*list.List),
	}

	for ptype, plugins := range oldState.Plugins {
		newState.Plugins[ptype] = make(map[string]*list.List)
		for pname, pluginInfo := range plugins {
			newState.Plugins[ptype][pname] = list.New()
			entry := list.Element{Value: pluginInfo}
			newState.Plugins[ptype][pname].PushFront(entry)
		}
	}
	return dynamicBkt.Put(registryStateKey, newState)
}
