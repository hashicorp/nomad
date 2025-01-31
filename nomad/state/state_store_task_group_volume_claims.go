// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertACLBindingRules is used to insert a task group volume assignment into
// the state store.
func (s *StateStore) UpsertTaskGroupVolumeClaim(
	index uint64, claim *structs.TaskGroupVolumeClaim) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(structs.TaskGroupVolumeClaimRegisterRequestType, index)
	defer txn.Abort()

	existingRaw, err := txn.First(TableTaskGroupVolumeClaim, indexID, claim.ID)
	if err != nil {
		return fmt.Errorf("Task group volume association lookup failed: %v", err)
	}

	var existing *structs.TaskGroupVolumeClaim
	if existingRaw != nil {
		existing = existingRaw.(*structs.TaskGroupVolumeClaim)
	}

	if existing != nil {
		if existing.Equal(claim) {
			return nil
		}

		claim.CreateIndex = existing.CreateIndex
		claim.ModifyIndex = index
		claim.CreateTime = existing.CreateTime
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

	return txn.Commit()
}

// DeleteTaskGroupVolumeClaim is responsible for deleting volume claims.
func (s *StateStore) DeleteTaskGroupVolumeClaim(index uint64, claimID string) error {
	txn := s.db.WriteTxnMsgT(structs.TaskGroupVolumeClaimDeleteRequestType, index)
	defer txn.Abort()

	existing, err := txn.First(TableTaskGroupVolumeClaim, indexID, claimID)
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

// GetTaskGroupVolumeClaims returns an iterator that contains all task group
// volume associations stored within state.
func (s *StateStore) GetTaskGroupVolumeClaims(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableTaskGroupVolumeClaim, indexID)
	if err != nil {
		return nil, fmt.Errorf("Task group volume claims lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}
