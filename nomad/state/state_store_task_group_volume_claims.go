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

	existingRaw, err := txn.First(TableTaskGroupVolumeClaim, indexID, association.ID)
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
	if err := txn.Insert(TableTaskGroupVolumeClaim, association); err != nil {
		return fmt.Errorf("Task group volume claim insert failed: %v", err)
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// DeleteACLBindingRules is responsible for batch deleting ACL binding rules.
// It uses a single write transaction for efficiency, however, any error means
// no entries will be committed. An error is produced if a rule is not found
// within state which has been passed within the array.
func (s *StateStore) DeleteTaskGroupVolumeAssociation(index uint64, bindingRuleIDs []string) error {
	txn := s.db.WriteTxnMsgT(structs.ACLBindingRulesDeleteRequestType, index)
	defer txn.Abort()

	for _, ruleID := range bindingRuleIDs {
		if err := s.deleteACLBindingRuleTxn(txn, ruleID); err != nil {
			return err
		}
	}

	// Update the index table to indicate an update has occurred.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLBindingRules, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// deleteACLBindingRuleTxn deletes a single ACL binding rule from the state
// store using the provided write transaction. It is the responsibility of the
// caller to update the index table.
func (s *StateStore) deleteTaskGroupVolumeAssociationTxn(txn *txn, ruleID string) error {
	existing, err := txn.First(TableACLBindingRules, indexID, ruleID)
	if err != nil {
		return fmt.Errorf("ACL binding rule lookup failed: %v", err)
	}
	if existing == nil {
		return errors.New("ACL binding rule not found")
	}

	// Delete the existing entry from the table.
	if err := txn.Delete(TableACLBindingRules, existing); err != nil {
		return fmt.Errorf("ACL binding rule deletion failed: %v", err)
	}
	return nil
}

// GetACLBindingRules returns an iterator that contains all ACL binding rules
// stored within state.
func (s *StateStore) GetTaskGroupVolumeAssociations(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table to get all ACL binding rules.
	iter, err := txn.Get(TableACLBindingRules, indexID)
	if err != nil {
		return nil, fmt.Errorf("ACL binding rules lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetACLBindingRule returns a single ACL binding rule specified by the input
// ID. The binding rule object will be nil, if no matching entry was found; it
// is the responsibility of the caller to check for this.
func (s *StateStore) GetTaskGroupVolumeAssociation(ws memdb.WatchSet, ruleID string) (*structs.ACLBindingRule, error) {
	txn := s.db.ReadTxn()

	// Perform the ACL binding rule lookup using the ID.
	watchCh, existing, err := txn.FirstWatch(TableACLBindingRules, indexID, ruleID)
	if err != nil {
		return nil, fmt.Errorf("ACL binding rule lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLBindingRule), nil
	}
	return nil, nil
}

// GetACLBindingRulesByAuthMethod returns an iterator with all binding rules
// associated with the named authentication method.
// func (s *StateStore) GetACLBindingRulesByAuthMethod(
// ws memdb.WatchSet, authMethod string) (memdb.ResultIterator, error) {
//
// txn := s.db.ReadTxn()
//
// iter, err := txn.Get(TableACLBindingRules, indexAuthMethod, authMethod)
// if err != nil {
// return nil, fmt.Errorf("ACL binding rule lookup failed: %v", err)
// }
// ws.Add(iter.WatchCh())
//
// return iter, nil
// }
