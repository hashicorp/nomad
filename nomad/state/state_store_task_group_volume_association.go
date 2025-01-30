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
func (s *StateStore) UpsertTaskGroupVolumeAssociation(
	index uint64, association *structs.TaskGroupVolumeClaim) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(structs.TaskGroupVolumeAssociationRegisterRequestType, index)
	defer txn.Abort()

	existingRaw, err := txn.First(TableTaskVolumeAssignment, indexID, association.ID)
	if err != nil {
		return fmt.Errorf("Task group volume association lookup failed: %v", err)
	}

	var existing *structs.TaskGroupVolumeClaim
	if existingRaw != nil {
		existing = existingRaw.(*structs.TaskGroupVolumeClaim)
	}

	if existing != nil {
		if existing.Equal(association) {
			return nil
		}

		association.CreateIndex = existing.CreateIndex
		association.ModifyIndex = index
		association.CreateTime = existing.CreateTime
	} else {
		association.CreateIndex = index
		association.ModifyIndex = index
	}

	// Insert the claim into the table.
	if err := txn.Insert(TableTaskVolumeAssignment, association); err != nil {
		return fmt.Errorf("Task group volume claim insert failed: %v", err)
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskVolumeAssignment, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}
