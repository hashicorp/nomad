// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *StateStore) UpsertTaskGroupVolumeClaim(index uint64, claim *structs.TaskGroupVolumeClaim) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(structs.TaskGroupVolumeClaimRegisterRequestType, index)
	defer txn.Abort()

	if err := s.upsertTaskGroupVolumeClaimImpl(index, claim, txn); err != nil {
		return err
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertTaskGroupVolumeClaimImpl is used to insert a task group volume claim into
// the state store.
func (s *StateStore) upsertTaskGroupVolumeClaimImpl(
	index uint64, claim *structs.TaskGroupVolumeClaim, txn *txn) error {

	existingRaw, err := txn.First(TableTaskGroupVolumeClaim, indexID, claim.Namespace, claim.JobID, claim.TaskGroupName, claim.VolumeID)
	if err != nil {
		return fmt.Errorf("Task group volume association lookup failed: %v", err)
	}

	var existing *structs.TaskGroupVolumeClaim
	if existingRaw != nil {
		existing = existingRaw.(*structs.TaskGroupVolumeClaim)
	}

	if existing != nil {
		// do allocation ID and volume ID match?
		if existing.ClaimedByAlloc(claim) {
			return nil
		}

		claim.CreateIndex = existing.CreateIndex
		claim.ModifyIndex = index
	} else {
		claim.CreateIndex = index
		claim.ModifyIndex = index
	}

	// Insert the claim into the table.
	if err := txn.Insert(TableTaskGroupVolumeClaim, claim); err != nil {
		return fmt.Errorf("Task group volume claim insert failed: %v", err)
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// DeleteTaskGroupVolumeClaim is responsible for deleting volume claims.
func (s *StateStore) DeleteTaskGroupVolumeClaim(index uint64, namespace, jobID, taskGroupName string) error {
	txn := s.db.WriteTxnMsgT(structs.TaskGroupVolumeClaimDeleteRequestType, index)
	defer txn.Abort()

	existing, err := txn.First(TableTaskGroupVolumeClaim, indexID, namespace, jobID, taskGroupName)
	if err != nil {
		return fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}
	if existing == nil {
		return errors.New("ACL binding rule not found")
	}

	// Delete the existing entry from the table.
	if err := txn.Delete(TableTaskGroupVolumeClaim, existing); err != nil {
		return fmt.Errorf("Task group volume claim deletion failed: %v", err)
	}

	// Update the index table to indicate an update has occurred.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// GetTaskGroupVolumeClaim returns a volume claim that matches the namespace,
// job id and task group name (there can be only one)
func (s *StateStore) GetTaskGroupVolumeClaim(ws memdb.WatchSet, namespace, jobID, taskGroupName, volumeID string) (*structs.TaskGroupVolumeClaim, error) {
	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch(TableTaskGroupVolumeClaim, indexID, namespace, jobID, taskGroupName, volumeID)
	if err != nil {
		return nil, fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.TaskGroupVolumeClaim), nil
	}

	return nil, nil
}
