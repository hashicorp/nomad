// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package state

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// quotaSpecExists on returns whether the quota exists
func (s *StateStore) quotaSpecExists(txn *txn, name string) (bool, error) {
	return false, nil
}

func (s *StateStore) quotaReconcile(index uint64, txn *txn, newQuota, oldQuota string) error {
	return nil
}

// updateEntWithAlloc is used to update Nomad Enterprise objects when an allocation is
// added/modified/deleted
func (s *StateStore) updateEntWithAlloc(index uint64, new, existing *structs.Allocation, txn *txn) error {
	return nil
}

// deleteRecommendationsByJob deletes all recommendations for the specified job
func (s *StateStore) deleteRecommendationsByJob(index uint64, txn Txn, job *structs.Job) error {
	return nil
}

// updateJobRecommendations updates/deletes job recommendations as necessary for a job update
func (s *StateStore) updateJobRecommendations(index uint64, txn Txn, prevJob, newJob *structs.Job) error {
	return nil
}
