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

// updateEntWithAlloc is used to update enterprise objects when an allocation is
// added/modified/deleted
func (s *StateStore) updateEntWithAlloc(index uint64, new, existing *structs.Allocation, txn *memdb.Txn) error {
	if err := s.updateQuotaWithAlloc(index, new, existing, txn); err != nil {
		return fmt.Errorf("updating quota usage failed: %v", err)
	}
	return nil
}

// updateQuotaWithAlloc is used to update quotas when an allocation is
// added/modified/deleted
func (s *StateStore) updateQuotaWithAlloc(index uint64, new, existing *structs.Allocation, txn *memdb.Txn) error {
	// This could only mean the allocation is being deleted and it should first
	// be set to a terminal status which means we would have already deducted
	// its resources.
	if new == nil {
		return nil
	}

	// Check if the namespace of the allocation has an attached quota.
	ns, err := s.namespaceByNameImpl(nil, txn, new.Namespace)
	if err != nil {
		return err
	} else if ns == nil {
		return fmt.Errorf("allocation %q is in non-existent namespace %q", new.ID, new.Namespace)
	} else if ns.Quota == "" {
		// Nothing to do
		return nil
	}

	// Get the quota usage object
	usage, err := s.quotaUsageByNameImpl(txn, nil, ns.Quota)
	if err != nil {
		return err
	} else if usage == nil {
		return fmt.Errorf("non-existent quota usage for spec %q", ns.Quota)
	}

	// Determine if we need to update the quota usage based on the allocation
	// change.
	var updated *structs.QuotaUsage
	if existing == nil {
		// This is an unexpected case, just guarding ourselves
		if new.TerminalStatus() {
			return nil
		}

		// We are just adding an allocation, so account for its resources
		updated = usage.Copy()
		limit := structs.FindRegionLimit(updated.Used, s.config.Region)
		if limit != nil {
			limit.Add(new)
		}
	} else if !existing.TerminalStatus() && new.TerminalStatus() {
		// Allocation is now terminal, so discount its resources
		updated = usage.Copy()
		limit := structs.FindRegionLimit(updated.Used, s.config.Region)
		if limit != nil {
			limit.Subtract(new)
		}
	}

	// No change so nothing to do
	if updated == nil {
		return nil
	}

	// Insert the updated quota usage
	if err := s.upsertQuotaUsageImpl(index, txn, updated, false); err != nil {
		return fmt.Errorf("upserting quota usage for spec %q failed: %v", usage.Name, err)
	}

	return nil
}

// upsertNamespaceImpl is used to upsert a namespace
func (s *StateStore) upsertNamespaceImpl(index uint64, txn *memdb.Txn, namespace *structs.Namespace) error {
	// Ensure the namespace hash is non-nil. This should be done outside the state store
	// for performance reasons, but we check here for defense in depth.
	ns := namespace
	if len(ns.Hash) == 0 {
		ns.SetHash()
	}

	// Check if the namespace already exists
	existing, err := txn.First(TableNamespaces, "id", ns.Name)
	if err != nil {
		return fmt.Errorf("namespace lookup failed: %v", err)
	}

	// Setup the indexes correctly and determine which quotas need to be
	// reconciled
	var oldQuota string
	if existing != nil {
		exist := existing.(*structs.Namespace)
		ns.CreateIndex = exist.CreateIndex
		ns.ModifyIndex = index

		// Grab the old quota on the namespace
		oldQuota = exist.Quota
	} else {
		ns.CreateIndex = index
		ns.ModifyIndex = index
	}

	// Validate that the quota on the new namespace exists
	if ns.Quota != "" {
		exists, err := s.quotaSpecExists(txn, ns.Quota)
		if err != nil {
			return fmt.Errorf("looking up namespace quota %q failed: %v", ns.Quota, err)
		} else if !exists {
			return fmt.Errorf("namespace %q using non-existent quota %q", ns.Name, ns.Quota)
		}
	}

	// Insert the namespace
	if err := txn.Insert(TableNamespaces, ns); err != nil {
		return fmt.Errorf("namespace insert failed: %v", err)
	}

	// Reconcile the quota usages if the namespace has changed the quota object
	// it is accounting against.
	//
	// OPTIMIZATION:
	// For now lets do the simple and safe thing and reconcile the full
	// quota. In the future we can optimize it to just scan the allocs in
	// the effected namespaces.

	// Existing  | New       | Action
	// "" or "a" | "" or "a" | None
	// ""        | "a"       | Reconcile "a"
	// "a"       | ""        | reconcile "a"
	// "a"       | "b"       | Reconcile "a" and "b"
	newQuota := ns.Quota
	var reconcileQuotas []string

	switch {
	case oldQuota == newQuota:
		// Do nothing
	case oldQuota == "" && newQuota != "":
		reconcileQuotas = append(reconcileQuotas, newQuota)
	case oldQuota != "" && newQuota == "":
		reconcileQuotas = append(reconcileQuotas, oldQuota)
	case oldQuota != newQuota:
		reconcileQuotas = append(reconcileQuotas, oldQuota, newQuota)
	}

	for _, q := range reconcileQuotas {
		// Get the spec object
		spec, err := s.quotaSpecByNameImpl(nil, txn, q)
		if err != nil {
			return fmt.Errorf("failed to lookup quota spec %q: %v", q, err)
		}
		if spec == nil {
			return fmt.Errorf("unknown quota spec %q", q)
		}

		// Get the usage object
		usage, err := s.quotaUsageByNameImpl(txn, nil, q)
		if err != nil {
			return fmt.Errorf("failed to lookup quota usage for spec %q: %v", q, err)
		}
		if usage == nil {
			return fmt.Errorf("nil quota usage for spec %q", q)
		}

		opts := reconcileQuotaUsageOpts{
			Usage:     usage,
			Spec:      spec,
			AllLimits: true,
		}
		if err := s.reconcileQuotaUsage(index, txn, opts); err != nil {
			return fmt.Errorf("reconciling quota usage for spec %q failed: %v", q, err)
		}

		// Update the quota after reconciling
		if err := txn.Insert(TableQuotaUsage, usage); err != nil {
			return fmt.Errorf("upserting quota usage failed: %v", err)
		}
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

			// Get the usage object
			usage, err := s.quotaUsageByNameImpl(txn, nil, spec.Name)
			if err != nil {
				return fmt.Errorf("failed to lookup quota usage for spec %q: %v", spec.Name, err)
			}
			if usage == nil {
				return fmt.Errorf("nil quota usage for spec %q", spec.Name)
			}

			opts := reconcileQuotaUsageOpts{
				Usage: usage,
				Spec:  spec,
			}
			if err := s.reconcileQuotaUsage(index, txn, opts); err != nil {
				return fmt.Errorf("reconciling quota usage for spec %q failed: %v", spec.Name, err)
			}

			// Update the quota after reconciling
			if err := txn.Insert(TableQuotaUsage, usage); err != nil {
				return fmt.Errorf("upserting quota usage failed: %v", err)
			}

		} else {
			spec.CreateIndex = index
			spec.ModifyIndex = index
		}

		// Update the quota
		if err := txn.Insert(TableQuotaSpec, spec); err != nil {
			return fmt.Errorf("upserting quota specification failed: %v", err)
		}

		// If we just created the spec, create the associated usage object. This
		// has to be done after inserting the spec.
		if spec.CreateIndex == spec.ModifyIndex {
			// Create the usage object for the new quota spec
			usage := structs.QuotaUsageFromSpec(spec)
			if err := s.upsertQuotaUsageImpl(index, txn, usage, true); err != nil {
				return fmt.Errorf("upserting quota usage for spec %q failed: %v", spec.Name, err)
			}
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
		// Ensure that there are no references to the quota spec
		iter, err := s.namespacesByQuotaImpl(nil, txn, name)
		if err != nil {
			return fmt.Errorf("looking up namespaces by quota failed: %v", err)
		}

		var namespaces []string
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ns := raw.(*structs.Namespace)
			namespaces = append(namespaces, ns.Name)
		}
		if len(namespaces) != 0 {
			return fmt.Errorf("Quota can't be removed since it is referenced by the following namespaces: %v", namespaces)
		}

		if _, err := txn.DeleteAll(TableQuotaSpec, "id", name); err != nil {
			return fmt.Errorf("deleting quota specification %q failed: %v", name, err)
		}

		// Remove the tracking usage object as well.
		if err := s.deleteQuotaUsageImpl(index, txn, name); err != nil {
			return fmt.Errorf("deleting quota usage for spec %q failed: %v", name, err)
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
	return s.quotaSpecByNameImpl(ws, txn, name)
}

func (s *StateStore) quotaSpecByNameImpl(ws memdb.WatchSet, txn *memdb.Txn, name string) (*structs.QuotaSpec, error) {
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

// quotaSpecExists on returns whether the quota exists
func (s *StateStore) quotaSpecExists(txn *memdb.Txn, name string) (bool, error) {
	qs, err := s.quotaSpecByNameImpl(nil, txn, name)
	return qs != nil, err
}

// QuotaSpecsByNamePrefix is used to lookup quota specifications by prefix
func (s *StateStore) QuotaSpecsByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
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

// TODO(alex): Potentially delete the the Upsert/Delete methods for Usages. They
// should be side effects of other calls.
// UpsertQuotaUsages is used to create or update a set of quota usages
func (s *StateStore) UpsertQuotaUsages(index uint64, usages []*structs.QuotaUsage) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, usage := range usages {
		if err := s.upsertQuotaUsageImpl(index, txn, usage, true); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

// upsertQuotaUsageImpl is used to create or update a quota usage object and
// potentially reconcile the usages.
func (s *StateStore) upsertQuotaUsageImpl(index uint64, txn *memdb.Txn, usage *structs.QuotaUsage,
	reconcile bool) error {

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

	if reconcile {
		// Retrieve the specification
		spec, err := s.quotaSpecByNameImpl(nil, txn, usage.Name)
		if err != nil {
			return fmt.Errorf("error retrieving quota specification %q: %v", usage.Name, err)
		} else if spec == nil {
			return fmt.Errorf("unknown quota specification %q", usage.Name)
		}

		opts := reconcileQuotaUsageOpts{
			Usage:     usage,
			Spec:      spec,
			AllLimits: true,
		}
		if err := s.reconcileQuotaUsage(index, txn, opts); err != nil {
			return fmt.Errorf("reconciling quota usage for spec %q failed: %v", spec.Name, err)
		}
	}

	// Update the quota
	if err := txn.Insert(TableQuotaUsage, usage); err != nil {
		return fmt.Errorf("upserting quota usage failed: %v", err)
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableQuotaUsage, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// reconcileQuotaUsageOpts is used to parameterize how a QuotaUsage is
// reconciled.
type reconcileQuotaUsageOpts struct {
	// Usage is the usage object to be reconciled in-place
	Usage *structs.QuotaUsage

	// AllLimits denotes whether all limits should be reconciled. If set to
	// false, only limits that have been added or changed from the passed spec
	// will be reconciled.
	AllLimits bool

	// Spec is the specification that the quota usage object should reflect.
	Spec *structs.QuotaSpec
}

// reconcileQuotaUsage computes the usage for the QuotaUsage object
func (s *StateStore) reconcileQuotaUsage(index uint64, txn *memdb.Txn, opts reconcileQuotaUsageOpts) error {
	if opts.Spec == nil || opts.Usage == nil {
		return fmt.Errorf("invalid quota usage reconcile options: %+v", opts)
	}

	// Grab the variables for easier referencing
	usage := opts.Usage
	spec := opts.Spec

	// Update the modify index
	usage.ModifyIndex = index

	// If all, remove all the existing limits so that when we diff we recreate
	// all
	if opts.AllLimits {
		usage.Used = make(map[string]*structs.QuotaLimit, len(spec.Limits))
	}

	// Diff the usage and the spec to determine what to add and delete
	create, remove := usage.DiffLimits(spec)

	// Remove what we don't need
	for _, rm := range remove {
		delete(usage.Used, string(rm.Hash))
	}

	// Filter the additions by if they apply to this region.
	filteredRegions := create[:0]
	region := s.config.Region
	for _, limit := range create {
		if limit.Region == region {
			filteredRegions = append(filteredRegions, limit)
		}
	}

	// In the future we will need more advanced filtering to determine if the
	// limit applies per allocation but for for now there should only be one
	// limit and all allocations from namespaces using the quota apply to it.
	if len(filteredRegions) == 0 {
		return nil
	}

	// Create a copy of the limit from the spec
	specLimit := filteredRegions[0]
	usageLimit := &structs.QuotaLimit{
		Region:      specLimit.Region,
		RegionLimit: &structs.Resources{},
		Hash:        specLimit.Hash,
	}

	// Determine the namespaces using the quota
	var namespaces []*structs.Namespace
	iter, err := s.namespacesByQuotaImpl(nil, txn, spec.Name)
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		ns := raw.(*structs.Namespace)
		namespaces = append(namespaces, ns)
	}

	for _, ns := range namespaces {
		allocs, err := s.allocsByNamespaceImpl(nil, txn, ns.Name)
		if err != nil {
			return err
		}

		for {
			raw := allocs.Next()
			if raw == nil {
				break
			}

			// Allocation counts against quota if not terminal
			alloc := raw.(*structs.Allocation)
			if !alloc.TerminalStatus() {
				usageLimit.Add(alloc)
			}
		}
	}

	usage.Used[string(usageLimit.Hash)] = usageLimit
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

// deleteQuotaUsageImpl deletes the quota usages with the given name
func (s *StateStore) deleteQuotaUsageImpl(index uint64, txn *memdb.Txn, name string) error {
	// Delete the quota usage
	if _, err := txn.DeleteAll(TableQuotaUsage, "id", name); err != nil {
		return fmt.Errorf("deleting quota usage failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{TableQuotaUsage, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return nil
}

// QuotaUsageByName is used to lookup a quota usage by name
func (s *StateStore) QuotaUsageByName(ws memdb.WatchSet, name string) (*structs.QuotaUsage, error) {
	txn := s.db.Txn(false)
	return s.quotaUsageByNameImpl(txn, ws, name)
}

func (s *StateStore) quotaUsageByNameImpl(txn *memdb.Txn, ws memdb.WatchSet, name string) (*structs.QuotaUsage, error) {
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

// QuotaUsagesByNamePrefix is used to lookup quota usages by prefix
func (s *StateStore) QuotaUsagesByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
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
