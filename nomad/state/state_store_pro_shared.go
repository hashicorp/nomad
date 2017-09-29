// +build pro ent

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// namespaceExists returns whether a namespace exists
func (s *StateStore) namespaceExists(txn *memdb.Txn, namespace string) (bool, error) {
	if namespace == structs.DefaultNamespace {
		return true, nil
	}

	existing, err := txn.First(TableNamespaces, "id", namespace)
	if err != nil {
		return false, fmt.Errorf("namespace lookup failed: %v", err)
	}

	return existing != nil, nil
}

// UpsertNamespace is used to register or update a set of namespaces
func (s *StateStore) UpsertNamespaces(index uint64, namespaces []*structs.Namespace) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, ns := range namespaces {
		if err := s.upsertNamespaceImpl(index, txn, ns); err != nil {
			return err
		}
	}

	if err := txn.Insert("index", &IndexEntry{TableNamespaces, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteNamespaces is used to remove a set of namespaces
func (s *StateStore) DeleteNamespaces(index uint64, names []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, name := range names {
		// Lookup the namespace
		existing, err := txn.First(TableNamespaces, "id", name)
		if err != nil {
			return fmt.Errorf("namespace lookup failed: %v", err)
		}
		if existing == nil {
			return fmt.Errorf("namespace not found")
		}

		ns := existing.(*structs.Namespace)
		if ns.Name == structs.DefaultNamespace {
			return fmt.Errorf("default namespace can not be deleted")
		}

		// Ensure that the namespace doesn't have any non-terminal jobs
		iter, err := s.jobsByNamespaceImpl(nil, name, txn)
		if err != nil {
			return err
		}

		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			job := raw.(*structs.Job)

			if job.Status != structs.JobStatusDead {
				return fmt.Errorf("namespace %q contains at least one non-terminal job %q. "+
					"All jobs must be terminal in namespace before it can be deleted", name, job.ID)
			}
		}

		// Delete the namespace
		if err := txn.Delete(TableNamespaces, existing); err != nil {
			return fmt.Errorf("namespace deletion failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{TableNamespaces, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// NamespaceByName is used to lookup a namespace by name
func (s *StateStore) NamespaceByName(ws memdb.WatchSet, name string) (*structs.Namespace, error) {
	txn := s.db.Txn(false)
	return s.namespaceByNameImpl(ws, txn, name)
}

// namespaceByNameImpl is used to lookup a namespace by name
func (s *StateStore) namespaceByNameImpl(ws memdb.WatchSet, txn *memdb.Txn, name string) (*structs.Namespace, error) {
	watchCh, existing, err := txn.FirstWatch(TableNamespaces, "id", name)
	if err != nil {
		return nil, fmt.Errorf("namespace lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Namespace), nil
	}
	return nil, nil
}

// NamespacesByNamePrefix is used to lookup namespaces by prefix
func (s *StateStore) NamespacesByNamePrefix(ws memdb.WatchSet, namePrefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get(TableNamespaces, "id_prefix", namePrefix)
	if err != nil {
		return nil, fmt.Errorf("namespaces lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// Namespaces returns an iterator over all the namespaces
func (s *StateStore) Namespaces(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire namespace table
	iter, err := txn.Get(TableNamespaces, "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// NamespacesByQuota is used to lookup namespaces by quota
func (s *StateStore) NamespacesByQuota(ws memdb.WatchSet, quota string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	return s.namespacesByQuotaImpl(ws, txn, quota)
}

// namespacesByQuotaImpl is used to lookup namespaces by quota
func (s *StateStore) namespacesByQuotaImpl(ws memdb.WatchSet, txn *memdb.Txn, quota string) (memdb.ResultIterator, error) {
	iter, err := txn.Get(TableNamespaces, "quota", quota)
	if err != nil {
		return nil, fmt.Errorf("namespaces lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// NamespaceRestore is used to restore a namespace
func (r *StateRestore) NamespaceRestore(ns *structs.Namespace) error {
	if err := r.txn.Insert(TableNamespaces, ns); err != nil {
		return fmt.Errorf("namespace insert failed: %v", err)
	}
	return nil
}
