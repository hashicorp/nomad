package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertACLAuthMethods is used to insert a number of ACL auth methods into the
// state store. It uses a single write transaction for efficiency, however, any
// error means no entries will be committed.
func (s *StateStore) UpsertACLAuthMethods(index uint64, aclAuthMethods []*structs.ACLAuthMethod) error {

	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(structs.ACLAuthMethodsUpsertRequestType, index)
	defer txn.Abort()

	// updated tracks whether any inserts have been made. This allows us to
	// skip updating the index table if we do not need to.
	var updated bool

	// Iterate the array of methods. In the event of a single error, all inserts
	// fail via the txn.Abort() defer.
	for _, method := range aclAuthMethods {

		methodUpdated, err := s.upsertACLAuthMethodTxn(index, txn, method)
		if err != nil {
			return err
		}

		// Ensure we track whether any inserts have been made.
		updated = updated || methodUpdated
	}

	// If we did not perform any inserts, exit early.
	if !updated {
		return nil
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLAuthMethods, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertACLAuthMethodTxn inserts a single ACL auth method into the state store
// using the provided write transaction. It is the responsibility of the caller
// to update the index table.
func (s *StateStore) upsertACLAuthMethodTxn(index uint64, txn *txn, method *structs.ACLAuthMethod) (bool, error) {

	// Ensure the method hash is not zero to provide defense in depth. This
	// should be done outside the state store, so we do not spend time here and
	// thus Raft, when it can be avoided.
	if len(method.Hash) == 0 {
		method.SetHash()
	}

	// This validation also happens within the RPC handler, but Raft latency
	// could mean that by the time the state call is invoked, another Raft
	// update has already written a method with the same name. We therefore
	// need to check we are not trying to create a method with an existing
	// name.
	existingRaw, err := txn.First(TableACLAuthMethods, indexID, method.Name)
	if err != nil {
		return false, fmt.Errorf("ACL auth method lookup failed: %v", err)
	}

	var existing *structs.ACLAuthMethod
	if existingRaw != nil {
		existing = existingRaw.(*structs.ACLAuthMethod)
	}

	// Depending on whether this is an initial create, or an update, we need to
	// check and set certain parameters. The most important is to ensure any
	// create index is carried over.
	if existing != nil {

		// If the method already exists, check whether the update contains any
		// difference. If it doesn't, we can avoid a state update as well as
		// updates to any blocking queries.
		if existing.Equal(method) {
			return false, nil
		}

		method.CreateIndex = existing.CreateIndex
		method.CreateTime = existing.CreateTime
		method.ModifyIndex = index
	} else {
		method.CreateIndex = index
		method.ModifyIndex = index
	}

	// Insert the auth method into the table.
	if err := txn.Insert(TableACLAuthMethods, method); err != nil {
		return false, fmt.Errorf("ACL auth method insert failed: %v", err)
	}
	return true, nil
}

// DeleteACLAuthMethods is responsible for batch deleting ACL methods. It uses
// a single write transaction for efficiency, however, any error means no
// entries will be committed. An error is produced if a method is not found
// within state which has been passed within the array.
func (s *StateStore) DeleteACLAuthMethods(index uint64, authMethodNames []string) error {
	txn := s.db.WriteTxnMsgT(structs.ACLAuthMethodsDeleteRequestType, index)
	defer txn.Abort()

	for _, methodName := range authMethodNames {
		if err := s.deleteACLAuthMethodTxn(txn, methodName); err != nil {
			return err
		}
	}

	// Update the index table to indicate an update has occurred.
	if err := txn.Insert(tableIndex, &IndexEntry{TableACLAuthMethods, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// deleteACLAuthMethodTxn deletes a single ACL method name from the state store
// using the provided write transaction. It is the responsibility of the caller
// to update the index table.
func (s *StateStore) deleteACLAuthMethodTxn(txn *txn, methodName string) error {
	existing, err := txn.First(TableACLAuthMethods, indexID, methodName)
	if err != nil {
		return fmt.Errorf("ACL auth method lookup failed: %v", err)
	}
	if existing == nil {
		return errors.New("ACL auth method not found")
	}

	// Delete the existing entry from the table.
	if err := txn.Delete(TableACLAuthMethods, existing); err != nil {
		return fmt.Errorf("ACL auth method deletion failed: %v", err)
	}
	return nil
}

// GetACLAuthMethods returns an iterator that contains all ACL auth methods
// stored within state.
func (s *StateStore) GetACLAuthMethods(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table to get all ACL auth methods.
	iter, err := txn.Get(TableACLAuthMethods, indexID)
	if err != nil {
		return nil, fmt.Errorf("ACL auth method lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetACLAuthMethodByName returns a single ACL auth method specified by the
// input name. The auth method object will be nil, if no matching entry was
// found; it is the responsibility of the caller to check for this.
func (s *StateStore) GetACLAuthMethodByName(ws memdb.WatchSet, authMethod string) (*structs.ACLAuthMethod, error) {
	txn := s.db.ReadTxn()

	// Perform the ACL auth method lookup using the "ID" index (which points to
	// "Name" column)
	watchCh, existing, err := txn.FirstWatch(TableACLAuthMethods, indexID, authMethod)
	if err != nil {
		return nil, fmt.Errorf("ACL auth method lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLAuthMethod), nil
	}
	return nil, nil
}
