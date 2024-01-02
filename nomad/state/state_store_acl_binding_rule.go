// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertACLBindingRules is used to insert a number of ACL binding rules into
// the state store. It uses a single write transaction for efficiency, however,
// any error means no entries will be committed.
func (s *StateStore) UpsertACLBindingRules(
	index uint64, bindingRules []*structs.ACLBindingRule, allowMissingAuthMethod bool) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(structs.ACLBindingRulesUpsertRequestType, index)
	defer txn.Abort()

	// updated tracks whether any inserts have been made. This allows us to
	// skip updating the index table if we do not need to.
	var updated bool

	// Iterate the array of rules. In the event of a single error, all inserts
	// fail via the txn.Abort() defer.
	for _, rule := range bindingRules {

		bindingRuleInserted, err := s.upsertACLBindingRuleTxn(index, txn, rule, allowMissingAuthMethod)
		if err != nil {
			return err
		}

		// Ensure we track whether any inserts have been made.
		updated = updated || bindingRuleInserted
	}

	// If we did not perform any inserts, exit early.
	if !updated {
		return nil
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLBindingRules, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertACLBindingRuleTxn inserts a single ACL binding rule into the state
// store using the provided write transaction. It is the responsibility of the
// caller to update the index table.
func (s *StateStore) upsertACLBindingRuleTxn(
	index uint64, txn *txn, rule *structs.ACLBindingRule, allowMissingAuthMethod bool) (bool, error) {

	// Ensure the rule hash is not zero to provide defense in depth. This
	// should be done outside the state store, so we do not spend time here and
	// thus Raft, when it can be avoided.
	if len(rule.Hash) == 0 {
		rule.SetHash()
	}

	// This validation also happens within the RPC handler, but Raft latency
	// could mean that by the time the state call is invoked, another Raft
	// update has the auth method detailed in binding rule. Therefore, check
	// again while in our write txn.
	if !allowMissingAuthMethod {
		method, err := s.GetACLAuthMethodByName(nil, rule.AuthMethod)
		if err != nil {
			return false, fmt.Errorf("ACL auth method lookup failed: %v", err)
		}
		if method == nil {
			return false, fmt.Errorf("ACL binding rule insert failed: ACL auth method not found")
		}
	}

	// This validation also happens within the RPC handler, but Raft latency
	// could mean that by the time the state call is invoked, another Raft
	// update has already written a method with the same name. We therefore
	// need to check we are not trying to create a rule with an existing ID.
	existingRaw, err := txn.First(TableACLBindingRules, indexID, rule.ID)
	if err != nil {
		return false, fmt.Errorf("ACL binding rule lookup failed: %v", err)
	}

	var existing *structs.ACLBindingRule
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLBindingRule)
	}

	// Depending on whether this is an initial create, or an update, we need to
	// check and set certain parameters. The most important is to ensure any
	// create index is carried over.
	if existing != nil {

		// If the rule already exists, check whether the update contains any
		// difference. If it doesn't, we can avoid a state update as well as
		// updates to any blocking queries.
		if existing.Equal(rule) {
			return false, nil
		}

		rule.CreateIndex = existing.CreateIndex
		rule.ModifyIndex = index
		rule.CreateTime = existing.CreateTime
	} else {
		rule.CreateIndex = index
		rule.ModifyIndex = index
	}

	// Insert the auth method into the table.
	if err := txn.Insert(TableACLBindingRules, rule); err != nil {
		return false, fmt.Errorf("ACL binding rule insert failed: %v", err)
	}
	return true, nil
}

// DeleteACLBindingRules is responsible for batch deleting ACL binding rules.
// It uses a single write transaction for efficiency, however, any error means
// no entries will be committed. An error is produced if a rule is not found
// within state which has been passed within the array.
func (s *StateStore) DeleteACLBindingRules(index uint64, bindingRuleIDs []string) error {
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
func (s *StateStore) deleteACLBindingRuleTxn(txn *txn, ruleID string) error {
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
func (s *StateStore) GetACLBindingRules(ws memdb.WatchSet) (memdb.ResultIterator, error) {
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
func (s *StateStore) GetACLBindingRule(ws memdb.WatchSet, ruleID string) (*structs.ACLBindingRule, error) {
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
func (s *StateStore) GetACLBindingRulesByAuthMethod(
	ws memdb.WatchSet, authMethod string) (memdb.ResultIterator, error) {

	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableACLBindingRules, indexAuthMethod, authMethod)
	if err != nil {
		return nil, fmt.Errorf("ACL binding rule lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}
