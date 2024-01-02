// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"bytes"
	"container/list"
	"fmt"
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper/boltdd"
	"github.com/hashicorp/nomad/nomad/structs"
	"go.etcd.io/bbolt"
)

// NeedsUpgrade returns true if the BoltDB needs upgrading or false if it is
// already up to date.
func NeedsUpgrade(bdb *bbolt.DB) (upgradeTo09, upgradeTo13 bool, err error) {
	upgradeTo09 = true
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
			upgradeTo09 = false
			return nil
		}
		if bytes.Equal(v, metaVersion) {
			upgradeTo09 = false
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

// UpgradeAllocs upgrades the boltdb schema. Example 0.8 schema:
//
//	allocations
//	  15d83e8a-74a2-b4da-3f17-ed5c12895ea8
//	    echo
//	      simple-all (342 bytes)
//	    alloc (2827 bytes)
//	    alloc-dir (166 bytes)
//	    immutable (15 bytes)
//	    mutable (1294 bytes)
func UpgradeAllocs(logger hclog.Logger, tx *boltdd.Tx) error {
	btx := tx.BoltTx()
	allocationsBucket := btx.Bucket(allocationsBucketName)
	if allocationsBucket == nil {
		// No state!
		return nil
	}

	// Gather alloc buckets and remove unexpected key/value pairs
	allocBuckets := [][]byte{}
	cur := allocationsBucket.Cursor()
	for k, v := cur.First(); k != nil; k, v = cur.Next() {
		if v != nil {
			logger.Warn("deleting unexpected key in state db",
				"key", string(k), "value_bytes", len(v),
			)

			if err := cur.Delete(); err != nil {
				return fmt.Errorf("error deleting unexpected key %q: %v", string(k), err)
			}
			continue
		}

		allocBuckets = append(allocBuckets, k)
	}

	for _, allocBucket := range allocBuckets {
		allocID := string(allocBucket)

		bkt := allocationsBucket.Bucket(allocBucket)
		if bkt == nil {
			// This should never happen as we just read the bucket.
			return fmt.Errorf("unexpected bucket missing %q", allocID)
		}

		allocLogger := logger.With("alloc_id", allocID)
		if err := upgradeAllocBucket(allocLogger, tx, bkt, allocID); err != nil {
			// Log and drop invalid allocs
			allocLogger.Error("dropping invalid allocation due to error while upgrading state",
				"error", err,
			)

			// If we can't delete the bucket something is seriously
			// wrong, fail hard.
			if err := allocationsBucket.DeleteBucket(allocBucket); err != nil {
				return fmt.Errorf("error deleting invalid allocation state: %v", err)
			}
		}
	}

	return nil
}

// upgradeAllocBucket upgrades an alloc bucket.
func upgradeAllocBucket(logger hclog.Logger, tx *boltdd.Tx, bkt *bbolt.Bucket, allocID string) error {
	allocFound := false
	taskBuckets := [][]byte{}
	cur := bkt.Cursor()
	for k, v := cur.First(); k != nil; k, v = cur.Next() {
		switch string(k) {
		case "alloc":
			// Alloc has not changed; leave it be
			allocFound = true
		case "alloc-dir":
			// Drop alloc-dir entries as they're no longer needed.
			cur.Delete()
		case "immutable":
			// Drop immutable state. Nothing from it needs to be
			// upgraded.
			cur.Delete()
		case "mutable":
			// Decode and upgrade
			if err := upgradeOldAllocMutable(tx, allocID, v); err != nil {
				return err
			}
			cur.Delete()
		default:
			if v != nil {
				logger.Warn("deleting unexpected state entry for allocation",
					"key", string(k), "value_bytes", len(v),
				)

				if err := cur.Delete(); err != nil {
					return err
				}

				continue
			}

			// Nested buckets are tasks
			taskBuckets = append(taskBuckets, k)
		}
	}

	// If the alloc entry was not found, abandon this allocation as the
	// state has been corrupted.
	if !allocFound {
		return fmt.Errorf("alloc entry not found")
	}

	// Upgrade tasks
	for _, taskBucket := range taskBuckets {
		taskName := string(taskBucket)
		taskLogger := logger.With("task_name", taskName)

		taskBkt := bkt.Bucket(taskBucket)
		if taskBkt == nil {
			// This should never happen as we just read the bucket.
			return fmt.Errorf("unexpected bucket missing %q", taskName)
		}

		oldState, err := upgradeTaskBucket(taskLogger, taskBkt)
		if err != nil {
			taskLogger.Warn("dropping invalid task due to error while upgrading state",
				"error", err,
			)

			// Delete the invalid task bucket and treat failures
			// here as unrecoverable errors.
			if err := bkt.DeleteBucket(taskBucket); err != nil {
				return fmt.Errorf("error deleting invalid task state for task %q: %v",
					taskName, err,
				)
			}
			continue
		}

		// Convert 0.8 task state to 0.9 task state
		localTaskState, err := oldState.Upgrade(allocID, taskName)
		if err != nil {
			taskLogger.Warn("dropping invalid task due to error while upgrading state",
				"error", err,
			)

			// Delete the invalid task bucket and treat failures
			// here as unrecoverable errors.
			if err := bkt.DeleteBucket(taskBucket); err != nil {
				return fmt.Errorf("error deleting invalid task state for task %q: %v",
					taskName, err,
				)
			}
			continue
		}

		// Insert the new task state
		if err := putTaskRunnerLocalStateImpl(tx, allocID, taskName, localTaskState); err != nil {
			return err
		}

		// Delete the old task bucket
		if err := bkt.DeleteBucket(taskBucket); err != nil {
			return err
		}

		taskLogger.Trace("upgraded", "from", oldState.Version)
	}

	return nil
}

// upgradeTaskBucket iterates over keys in a task bucket, deleting invalid keys
// and returning the 0.8 version of the state.
func upgradeTaskBucket(logger hclog.Logger, bkt *bbolt.Bucket) (*taskRunnerState08, error) {
	simpleFound := false
	var trState taskRunnerState08

	cur := bkt.Cursor()
	for k, v := cur.First(); k != nil; k, v = cur.Next() {
		if v == nil {
			// value is nil: delete unexpected bucket
			logger.Warn("deleting unexpected task state bucket",
				"bucket", string(k),
			)

			if err := bkt.DeleteBucket(k); err != nil {
				return nil, fmt.Errorf("error deleting unexpected task bucket %q: %v", string(k), err)
			}
			continue
		}

		if !bytes.Equal(k, []byte("simple-all")) {
			// value is non-nil: delete unexpected entry
			logger.Warn("deleting unexpected task state entry",
				"key", string(k), "value_bytes", len(v),
			)

			if err := cur.Delete(); err != nil {
				return nil, fmt.Errorf("error delting unexpected task key %q: %v", string(k), err)
			}
			continue
		}

		// Decode simple-all
		simpleFound = true
		if err := codec.NewDecoderBytes(v, structs.MsgpackHandle).Decode(&trState); err != nil {
			return nil, fmt.Errorf("failed to decode task state from 'simple-all' entry: %v", err)
		}
	}

	if !simpleFound {
		return nil, fmt.Errorf("task state entry not found")
	}

	return &trState, nil
}

// upgradeOldAllocMutable upgrades Nomad 0.8 alloc runner state.
func upgradeOldAllocMutable(tx *boltdd.Tx, allocID string, oldBytes []byte) error {
	var oldMutable allocRunnerMutableState08
	err := codec.NewDecoderBytes(oldBytes, structs.MsgpackHandle).Decode(&oldMutable)
	if err != nil {
		return err
	}

	// Upgrade Deployment Status
	if err := putDeploymentStatusImpl(tx, allocID, oldMutable.DeploymentStatus); err != nil {
		return err
	}

	// Upgrade Task States
	for taskName, taskState := range oldMutable.TaskStates {
		if err := putTaskStateImpl(tx, allocID, taskName, taskState); err != nil {
			return err
		}
	}

	return nil
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
