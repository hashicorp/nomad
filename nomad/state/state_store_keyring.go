// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertRootKeyMeta saves root key meta or updates it in-place.
func (s *StateStore) UpsertRootKeyMeta(index uint64, rootKeyMeta *structs.RootKeyMeta, rekey bool) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// get any existing key for updating
	raw, err := txn.First(TableRootKeyMeta, indexID, rootKeyMeta.KeyID)
	if err != nil {
		return fmt.Errorf("root key metadata lookup failed: %v", err)
	}

	isRotation := false

	if raw != nil {
		existing := raw.(*structs.RootKeyMeta)
		rootKeyMeta.CreateIndex = existing.CreateIndex
		rootKeyMeta.CreateTime = existing.CreateTime
		isRotation = !existing.IsActive() && rootKeyMeta.IsActive()
	} else {
		rootKeyMeta.CreateIndex = index
		isRotation = rootKeyMeta.IsActive()
	}
	rootKeyMeta.ModifyIndex = index

	if rekey && !isRotation {
		return fmt.Errorf("cannot rekey without setting the new key active")
	}

	// if the upsert is for a newly-active key, we need to set all the
	// other keys as inactive in the same transaction.
	if isRotation {
		iter, err := txn.Get(TableRootKeyMeta, indexID)
		if err != nil {
			return err
		}
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			key := raw.(*structs.RootKeyMeta)
			modified := false

			switch key.State {
			case structs.RootKeyStateInactive:
				if rekey {
					key = key.MakeRekeying()
					modified = true
				}
			case structs.RootKeyStateActive:
				if rekey {
					key = key.MakeRekeying()
				} else {
					key = key.MakeInactive()
				}
				modified = true
			case structs.RootKeyStateRekeying, structs.RootKeyStateDeprecated:
				// nothing to do
			}

			if modified {
				key.ModifyIndex = index
				if err := txn.Insert(TableRootKeyMeta, key); err != nil {
					return err
				}
			}

		}
	}

	if err := txn.Insert(TableRootKeyMeta, rootKeyMeta); err != nil {
		return err
	}

	// update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableRootKeyMeta, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return txn.Commit()
}

// DeleteRootKeyMeta deletes a single root key, or returns an error if
// it doesn't exist.
func (s *StateStore) DeleteRootKeyMeta(index uint64, keyID string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// find the old key
	existing, err := txn.First(TableRootKeyMeta, indexID, keyID)
	if err != nil {
		return fmt.Errorf("root key metadata lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("root key metadata not found")
	}
	if err := txn.Delete(TableRootKeyMeta, existing); err != nil {
		return fmt.Errorf("root key metadata delete failed: %v", err)
	}

	// update the indexes table
	if err := txn.Insert("index", &IndexEntry{TableRootKeyMeta, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// RootKeyMetas returns an iterator over all root key metadata
func (s *StateStore) RootKeyMetas(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableRootKeyMeta, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// RootKeyMetaByID returns a specific root key meta
func (s *StateStore) RootKeyMetaByID(ws memdb.WatchSet, id string) (*structs.RootKeyMeta, error) {
	txn := s.db.ReadTxn()

	watchCh, raw, err := txn.FirstWatch(TableRootKeyMeta, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("root key metadata lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if raw != nil {
		return raw.(*structs.RootKeyMeta), nil
	}
	return nil, nil
}

// GetActiveRootKeyMeta returns the metadata for the currently active root key
func (s *StateStore) GetActiveRootKeyMeta(ws memdb.WatchSet) (*structs.RootKeyMeta, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableRootKeyMeta, indexID)
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		key := raw.(*structs.RootKeyMeta)
		if key.IsActive() {
			return key, nil
		}
	}
	return nil, nil
}

// IsRootKeyInUse determines whether a key has been used to sign a workload
// identity for a live allocation or encrypt any variables
func (s *StateStore) IsRootKeyInUse(keyID string) (bool, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableAllocs, indexSigningKey, keyID, true)
	if err != nil {
		return false, err
	}
	alloc := iter.Next()
	if alloc != nil {
		return true, nil
	}

	iter, err = txn.Get(TableVariables, indexKeyID, keyID)
	if err != nil {
		return false, err
	}
	variable := iter.Next()
	if variable != nil {
		return true, nil
	}

	return false, nil
}
