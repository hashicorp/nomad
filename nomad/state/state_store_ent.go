// +build ent

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertSentinelPolicies is used to create or update a set of Sentinel policies
func (s *StateStore) UpsertSentinelPolicies(index uint64, policies []*structs.SentinelPolicy) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, policy := range policies {
		// Ensure the policy hash is non-nil. This should be done outside the state store
		// for performance reasons, but we check here for defense in depth.
		if len(policy.Hash) == 0 {
			policy.SetHash()
		}

		// Check if the policy already exists
		existing, err := txn.First(TableSentinelPolicies, "id", policy.Name)
		if err != nil {
			return fmt.Errorf("policy lookup failed: %v", err)
		}

		// Update all the indexes
		if existing != nil {
			policy.CreateIndex = existing.(*structs.SentinelPolicy).CreateIndex
			policy.ModifyIndex = index
		} else {
			policy.CreateIndex = index
			policy.ModifyIndex = index
		}

		// Update the policy
		if err := txn.Insert(TableSentinelPolicies, policy); err != nil {
			return fmt.Errorf("upserting policy failed: %v", err)
		}
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableSentinelPolicies, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteSentinelPolicies deletes the policies with the given names
func (s *StateStore) DeleteSentinelPolicies(index uint64, names []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the policy
	for _, name := range names {
		if _, err := txn.DeleteAll(TableSentinelPolicies, "id", name); err != nil {
			return fmt.Errorf("deleting sentinel policy failed: %v", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{TableSentinelPolicies, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// SentinelPolicyByName is used to lookup a policy by name
func (s *StateStore) SentinelPolicyByName(ws memdb.WatchSet, name string) (*structs.SentinelPolicy, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch(TableSentinelPolicies, "id", name)
	if err != nil {
		return nil, fmt.Errorf("sentinel policy lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.SentinelPolicy), nil
	}
	return nil, nil
}

// SentinelPolicyByNamePrefix is used to lookup policies by prefix
func (s *StateStore) SentinelPolicyByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get(TableSentinelPolicies, "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("sentinel policy lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// SentinelPolicies returns an iterator over all the sentinel policies
func (s *StateStore) SentinelPolicies(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get(TableSentinelPolicies, "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// SentinelPoliciesByScope returns an iterator over all the sentinel policies by scope
func (s *StateStore) SentinelPoliciesByScope(ws memdb.WatchSet, scope string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get(TableSentinelPolicies, "scope", scope)
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// SentinelPolicyRestore is used to restore an Sentinel policy
func (r *StateRestore) SentinelPolicyRestore(policy *structs.SentinelPolicy) error {
	if err := r.txn.Insert(TableSentinelPolicies, policy); err != nil {
		return fmt.Errorf("inserting sentinel policy failed: %v", err)
	}
	return nil
}

// UpsertQuotaSpecs is used to create or update a set of quota specifications
func (s *StateStore) UpsertQuotaSpecs(index uint64, specs []*structs.QuotaSpec) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, spec := range specs {
		// Ensure the spec hash is non-nil. This should be done outside the state store
		// for performance reasons, but we check here for defense in depth.
		if len(spec.Hash) == 0 {
			spec.SetHash()
		}

		// Check if the spec already exists
		existing, err := txn.First(TableQuotaSpec, "id", spec.Name)
		if err != nil {
			return fmt.Errorf("quota specification lookup failed: %v", err)
		}

		// Update all the indexes
		if existing != nil {
			spec.CreateIndex = existing.(*structs.QuotaSpec).CreateIndex
			spec.ModifyIndex = index
		} else {
			spec.CreateIndex = index
			spec.ModifyIndex = index
		}

		// Update the quota
		if err := txn.Insert(TableQuotaSpec, spec); err != nil {
			return fmt.Errorf("upserting quota specification failed: %v", err)
		}
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableQuotaSpec, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteQuotaSpecs deletes the quota specifications with the given names
func (s *StateStore) DeleteQuotaSpecs(index uint64, names []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the quota specs
	for _, name := range names {
		if _, err := txn.DeleteAll(TableQuotaSpec, "id", name); err != nil {
			return fmt.Errorf("deleting quota specification failed: %v", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{TableQuotaSpec, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// QuotaSpecByName is used to lookup a quota specification by name
func (s *StateStore) QuotaSpecByName(ws memdb.WatchSet, name string) (*structs.QuotaSpec, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch(TableQuotaSpec, "id", name)
	if err != nil {
		return nil, fmt.Errorf("quota specification lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.QuotaSpec), nil
	}
	return nil, nil
}

// QuotaSpecByNamePrefix is used to lookup quota specifications by prefix
func (s *StateStore) QuotaSpecByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get(TableQuotaSpec, "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("quota specification lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// QuotaSpecs returns an iterator over all the quota specifications
func (s *StateStore) QuotaSpecs(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get(TableQuotaSpec, "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// QuotaSpecRestore is used to restore a quota specification
func (r *StateRestore) QuotaSpecRestore(spec *structs.QuotaSpec) error {
	if err := r.txn.Insert(TableQuotaSpec, spec); err != nil {
		return fmt.Errorf("inserting quota specification failed: %v", err)
	}
	return nil
}

// UpsertQuotaUsages is used to create or update a set of quota usages
func (s *StateStore) UpsertQuotaUsages(index uint64, usages []*structs.QuotaUsage) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, usage := range usages {
		// Check if the usage already exists
		existing, err := txn.First(TableQuotaUsage, "id", usage.Name)
		if err != nil {
			return fmt.Errorf("quota usage lookup failed: %v", err)
		}

		// Update all the indexes
		if existing != nil {
			usage.CreateIndex = existing.(*structs.QuotaUsage).CreateIndex
			usage.ModifyIndex = index
		} else {
			usage.CreateIndex = index
			usage.ModifyIndex = index
		}

		// Update the quota
		if err := txn.Insert(TableQuotaUsage, usage); err != nil {
			return fmt.Errorf("upserting quota usage failed: %v", err)
		}
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableQuotaUsage, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteQuotaUsages deletes the quota usages with the given names
func (s *StateStore) DeleteQuotaUsages(index uint64, names []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the quota usages
	for _, name := range names {
		if _, err := txn.DeleteAll(TableQuotaUsage, "id", name); err != nil {
			return fmt.Errorf("deleting quota usage failed: %v", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{TableQuotaUsage, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// QuotaUsageByName is used to lookup a quota usage by name
func (s *StateStore) QuotaUsageByName(ws memdb.WatchSet, name string) (*structs.QuotaUsage, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch(TableQuotaUsage, "id", name)
	if err != nil {
		return nil, fmt.Errorf("quota usage lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.QuotaUsage), nil
	}
	return nil, nil
}

// QuotaUsageByNamePrefix is used to lookup quota usages by prefix
func (s *StateStore) QuotaUsageByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get(TableQuotaUsage, "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("quota usages lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// QuotaUsages returns an iterator over all the quota usages
func (s *StateStore) QuotaUsages(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get(TableQuotaUsage, "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// QuotaUsageRestore is used to restore a quota usage
func (r *StateRestore) QuotaUsageRestore(usage *structs.QuotaUsage) error {
	if err := r.txn.Insert(TableQuotaUsage, usage); err != nil {
		return fmt.Errorf("inserting quota usage failed: %v", err)
	}
	return nil
}
