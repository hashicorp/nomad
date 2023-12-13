// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertServiceRegistrations is used to insert a number of service
// registrations into the state store. It uses a single write transaction for
// efficiency, however, any error means no entries will be committed.
func (s *StateStore) UpsertServiceRegistrations(
	msgType structs.MessageType, index uint64, services []*structs.ServiceRegistration) error {

	// Grab a write transaction, so we can use this across all service inserts.
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// updated tracks whether any inserts have been made. This allows us to
	// skip updating the index table if we do not need to.
	var updated bool

	// Iterate the array of services. In the event of a single error, all
	// inserts fail via the txn.Abort() defer.
	for _, service := range services {
		serviceUpdated, err := s.upsertServiceRegistrationTxn(index, txn, service)
		if err != nil {
			return err
		}
		// Ensure we track whether any inserts have been made.
		updated = updated || serviceUpdated
	}

	// If we did not perform any inserts, exit early.
	if !updated {
		return nil
	}

	// Perform the index table update to mark the new inserts.
	if err := txn.Insert(tableIndex, &IndexEntry{TableServiceRegistrations, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertServiceRegistrationTxn inserts a single service registration into the
// state store using the provided write transaction. It is the responsibility
// of the caller to update the index table.
func (s *StateStore) upsertServiceRegistrationTxn(
	index uint64, txn *txn, service *structs.ServiceRegistration) (bool, error) {

	existing, err := txn.First(TableServiceRegistrations, indexID, service.Namespace, service.ID)
	if err != nil {
		return false, fmt.Errorf("service registration lookup failed: %v", err)
	}

	// Set up the indexes correctly to ensure existing indexes are maintained.
	if existing != nil {
		exist := existing.(*structs.ServiceRegistration)
		if exist.Equal(service) {
			return false, nil
		}
		service.CreateIndex = exist.CreateIndex
		service.ModifyIndex = index
	} else {
		service.CreateIndex = index
		service.ModifyIndex = index
	}

	// Insert the service registration into the table.
	if err := txn.Insert(TableServiceRegistrations, service); err != nil {
		return false, fmt.Errorf("service registration insert failed: %v", err)
	}
	return true, nil
}

// DeleteServiceRegistrationByID is responsible for deleting a single service
// registration based on it's ID and namespace. If the service registration is
// not found within state, an error will be returned.
func (s *StateStore) DeleteServiceRegistrationByID(
	msgType structs.MessageType, index uint64, namespace, id string) error {

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	if err := s.deleteServiceRegistrationByIDTxn(index, txn, namespace, id); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *StateStore) deleteServiceRegistrationByIDTxn(
	index uint64, txn *txn, namespace, id string) error {

	// Lookup the service registration by its ID and namespace. This is a
	// unique index and therefore there will be a maximum of one entry.
	existing, err := txn.First(TableServiceRegistrations, indexID, namespace, id)
	if err != nil {
		return fmt.Errorf("service registration lookup failed: %v", err)
	}
	if existing == nil {
		return errors.New("service registration not found")
	}

	// Delete the existing entry from the table.
	if err := txn.Delete(TableServiceRegistrations, existing); err != nil {
		return fmt.Errorf("service registration deletion failed: %v", err)
	}

	// Update the index table to indicate an update has occurred.
	if err := txn.Insert(tableIndex, &IndexEntry{TableServiceRegistrations, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return nil
}

// DeleteServiceRegistrationByNodeID deletes all service registrations that
// belong on a single node. If there are no registrations tied to the nodeID,
// the call will noop without an error.
func (s *StateStore) DeleteServiceRegistrationByNodeID(
	msgType structs.MessageType, index uint64, nodeID string) error {

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	num, err := txn.DeleteAll(TableServiceRegistrations, indexNodeID, nodeID)
	if err != nil {
		return fmt.Errorf("deleting service registrations failed: %v", err)
	}

	// If we did not delete any entries, do not update the index table.
	// Otherwise, update the table with the latest index.
	switch num {
	case 0:
		return nil
	default:
		if err := txn.Insert(tableIndex, &IndexEntry{TableServiceRegistrations, index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	return txn.Commit()
}

// GetServiceRegistrations returns an iterator that contains all service
// registrations stored within state. This is primarily useful when performing
// listings which use the namespace wildcard operator. The caller is
// responsible for ensuring ACL access is confirmed, or filtering is performed
// before responding.
func (s *StateStore) GetServiceRegistrations(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableServiceRegistrations, indexID)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// GetServiceRegistrationsByNamespace returns an iterator that contains all
// registrations belonging to the provided namespace.
func (s *StateStore) GetServiceRegistrationsByNamespace(
	ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableServiceRegistrations, indexID+"_prefix", namespace, "")
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetServiceRegistrationByName returns an iterator that contains all service
// registrations whose namespace and name match the input parameters. This func
// therefore represents how to identify a single, collection of services that
// are logically grouped together.
func (s *StateStore) GetServiceRegistrationByName(
	ws memdb.WatchSet, namespace, name string) (memdb.ResultIterator, error) {

	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableServiceRegistrations, indexServiceName, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetServiceRegistrationByID returns a single registration. The registration
// will be nil, if no matching entry was found; it is the responsibility of the
// caller to check for this.
func (s *StateStore) GetServiceRegistrationByID(
	ws memdb.WatchSet, namespace, id string) (*structs.ServiceRegistration, error) {

	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch(TableServiceRegistrations, indexID, namespace, id)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ServiceRegistration), nil
	}
	return nil, nil
}

// GetServiceRegistrationsByAllocID returns an iterator containing all the
// service registrations corresponding to a single allocation.
func (s *StateStore) GetServiceRegistrationsByAllocID(
	ws memdb.WatchSet, allocID string) (memdb.ResultIterator, error) {

	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableServiceRegistrations, indexAllocID, allocID)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetServiceRegistrationsByJobID returns an iterator containing all the
// service registrations corresponding to a single job.
func (s *StateStore) GetServiceRegistrationsByJobID(
	ws memdb.WatchSet, namespace, jobID string) (memdb.ResultIterator, error) {

	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableServiceRegistrations, indexJob, namespace, jobID)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetServiceRegistrationsByNodeID identifies all service registrations tied to
// the specified nodeID. This is useful for performing an in-memory lookup in
// order to avoid calling DeleteServiceRegistrationByNodeID via a Raft message.
func (s *StateStore) GetServiceRegistrationsByNodeID(
	ws memdb.WatchSet, nodeID string) ([]*structs.ServiceRegistration, error) {

	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableServiceRegistrations, indexNodeID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("service registration lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	var result []*structs.ServiceRegistration
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		result = append(result, raw.(*structs.ServiceRegistration))
	}

	return result, nil
}
