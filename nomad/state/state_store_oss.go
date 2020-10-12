// +build !ent

package state

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// enterpriseInit is used to initialize the state store with enterprise
// objects.
func (s *StateStore) enterpriseInit() error {
	return nil
}

// namespaceExists returns whether a namespace exists
func (s *StateStore) namespaceExists(txn *txn, namespace string) (bool, error) {
	return namespace == structs.DefaultNamespace, nil
}

// updateEntWithAlloc is used to update Nomad Enterprise objects when an allocation is
// added/modified/deleted
func (s *StateStore) updateEntWithAlloc(index uint64, new, existing *structs.Allocation, txn *txn) error {
	return nil
}

func (s *StateStore) NamespaceNames() ([]string, error) {
	return []string{structs.DefaultNamespace}, nil
}
