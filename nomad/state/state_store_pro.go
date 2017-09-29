// +build pro

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// updateEntWithAlloc is used to update Nomad Pro objects when an allocation is
// added/modified/deleted
func (s *StateStore) updateEntWithAlloc(index uint64, new, existing *structs.Allocation, txn *memdb.Txn) error {
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

	// Setup the indexes correctly
	if existing != nil {
		exist := existing.(*structs.Namespace)
		ns.CreateIndex = exist.CreateIndex
		ns.ModifyIndex = index
	} else {
		ns.CreateIndex = index
		ns.ModifyIndex = index
	}

	// Validate that the quota exists
	if ns.Quota != "" {
		return fmt.Errorf("Namespaces can only attach quotas in Nomad Premium")
	}

	// Insert the namespace
	if err := txn.Insert(TableNamespaces, ns); err != nil {
		return fmt.Errorf("namespace insert failed: %v", err)
	}

	return nil
}
