// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertTaskGroupHostVolumeClaim is used to upsert claims into the state store.
// This method is only used in unit tests.
func (s *StateStore) UpsertTaskGroupHostVolumeClaim(msgType structs.MessageType, index uint64, claim *structs.TaskGroupHostVolumeClaim) error {
	// Grab a write transaction.
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()
	if err := s.upsertTaskGroupHostVolumeClaimImpl(index, claim, txn); err != nil {
		return err
	}

	return txn.Commit()
}

// upsertTaskGroupHostVolumeClaimImpl is used to insert a task group volume claim into
// the state store.
func (s *StateStore) upsertTaskGroupHostVolumeClaimImpl(
	index uint64, claim *structs.TaskGroupHostVolumeClaim, txn *txn) error {

	existingRaw, err := txn.First(TableTaskGroupHostVolumeClaim, indexID, claim.Namespace, claim.JobID, claim.TaskGroupName, claim.VolumeID)
	if err != nil {
		return fmt.Errorf("Task group volume association lookup failed: %v", err)
	}

	var existing *structs.TaskGroupHostVolumeClaim
	if existingRaw != nil {
		existing = existingRaw.(*structs.TaskGroupHostVolumeClaim)
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
	if err := txn.Insert(TableTaskGroupHostVolumeClaim, claim); err != nil {
		return fmt.Errorf("Task group volume claim insert failed: %v", err)
	}

	// Perform the index table update to mark the new insert.
	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupHostVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// GetTaskGroupHostVolumeClaim returns a volume claim that matches the namespace,
// job id and task group name (there can be only one)
func (s *StateStore) GetTaskGroupHostVolumeClaim(ws memdb.WatchSet, namespace, jobID, taskGroupName, volumeID string) (*structs.TaskGroupHostVolumeClaim, error) {
	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch(TableTaskGroupHostVolumeClaim, indexID, namespace, jobID, taskGroupName, volumeID)
	if err != nil {
		return nil, fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.TaskGroupHostVolumeClaim), nil
	}

	return nil, nil
}

// GetTaskGroupHostVolumeClaims returns all volume claims
func (s *StateStore) GetTaskGroupHostVolumeClaims(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableTaskGroupHostVolumeClaim, indexID)
	if err != nil {
		return nil, fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// TgvcSearchableFields lists fields that task group volume claims can be
// searched by
type TgvcSearchableFields struct {
	Namespace     string
	JobID         string
	TaskGroupName string
	VolumeName    string
}

// TaskGroupHostVolumeClaimsByFields returns all claims that match the fields,
// and handles namespace wildcards
func (s *StateStore) TaskGroupHostVolumeClaimsByFields(ws memdb.WatchSet, fields TgvcSearchableFields) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableTaskGroupHostVolumeClaim, indexID)
	if err != nil {
		return nil, fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	filter := memdb.NewFilterIterator(iter, func(raw interface{}) bool {
		claim, ok := raw.(*structs.TaskGroupHostVolumeClaim)
		if !ok {
			return true
		}

		// check which fields we should filter by
		if fields.Namespace != structs.AllNamespacesSentinel && fields.Namespace != "" {
			if claim.Namespace != fields.Namespace {
				return true
			}
		}
		if fields.JobID != "" && claim.JobID != fields.JobID {
			return true
		}
		if fields.TaskGroupName != "" && claim.TaskGroupName != fields.TaskGroupName {
			return true
		}
		if fields.VolumeName != "" && claim.VolumeName != fields.VolumeName {
			return true
		}
		return false
	})

	return filter, nil
}

// deleteTaskGroupHostVolumeClaimByNamespaceAndJob deletes all claims for a
// given namespace and job ID
func (s *StateStore) deleteTaskGroupHostVolumeClaimByNamespaceAndJob(index uint64, txn *txn, namespace, jobID string) error {
	iter, err := txn.Get(TableTaskGroupHostVolumeClaim, indexID)
	if err != nil {
		return fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		if claim.JobID == jobID && claim.Namespace == namespace {
			if err := txn.Delete(TableTaskGroupHostVolumeClaim, claim); err != nil {
				return fmt.Errorf("Task group volume claim deletion failed: %v", err)
			}
		}
	}

	return nil
}

// DeleteTaskGroupHostVolumeClaim deletes a claim by its ID
func (s *StateStore) DeleteTaskGroupHostVolumeClaim(index uint64, claimID string) error {
	txn := s.db.WriteTxnMsgT(structs.TaskGroupHostVolumeClaimDeleteRequestType, index)
	defer txn.Abort()

	obj, err := txn.First(TableTaskGroupHostVolumeClaim, indexClaimID, claimID)
	if err != nil {
		return fmt.Errorf("Task group volume claim lookup failed: %v", err)
	}

	if obj == nil {
		return errors.New("Task group volume claim does not exist")
	}

	if err := txn.Delete(TableTaskGroupHostVolumeClaim, obj); err != nil {
		return err
	}

	if err := txn.Insert(tableIndex, &IndexEntry{TableTaskGroupHostVolumeClaim, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}
