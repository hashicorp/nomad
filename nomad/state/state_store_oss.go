// +build !pro,!ent

package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// namespaceExists returns whether a namespace exists
func (s *StateStore) namespaceExists(txn *memdb.Txn, namespace string) (bool, error) {
	return namespace == structs.DefaultNamespace, nil
}

// updateEntWithAlloc is used to update Nomad Enterprise objects when an allocation is
// added/modified/deleted
func (s *StateStore) updateEntWithAlloc(index uint64, new, existing *structs.Allocation, txn *memdb.Txn) error {
	return nil
}
