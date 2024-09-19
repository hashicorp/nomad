// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertRootKey saves a root key or updates it in place.
func (s *StateStore) UpsertRootKey(index uint64, rootKey *structs.RootKey, rekey bool) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// get any existing key for updating
	raw, err := txn.First(TableRootKeys, indexID, rootKey.KeyID)
	if err != nil {
		return fmt.Errorf("root key lookup failed: %v", err)
	}

	isRotation := false

	if raw != nil {
		existing := raw.(*structs.RootKey)
		rootKey.CreateIndex = existing.CreateIndex
		rootKey.CreateTime = existing.CreateTime
		isRotation = !existing.IsActive() && rootKey.IsActive()
	} else {
		rootKey.CreateIndex = index
		isRotation = rootKey.IsActive()
	}
	rootKey.ModifyIndex = index

	if rekey && !isRotation {
		return fmt.Errorf("cannot rekey without setting the new key active")
	}

	// if the upsert is for a newly-active key, we need to set all the
	// other keys as inactive in the same transaction.
	if isRotation {
		iter, err := txn.Get(TableRootKeys, indexID)
		if err != nil {
			return err
		}
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			key := raw.(*structs.RootKey)
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
				if err := txn.Insert(TableRootKeys, key); err != nil {
					return err
				}

			}
		}
	}

	if err := txn.Insert(TableRootKeys, rootKey); err != nil {
		return err
	}
	if err := txn.Insert("index", &IndexEntry{TableRootKeys, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// DeleteRootKey deletes a single wrapped root key set, or returns an
// error if it doesn't exist.
func (s *StateStore) DeleteRootKey(index uint64, keyID string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// find the old key
	existing, err := txn.First(TableRootKeys, indexID, keyID)
	if err != nil {
		return fmt.Errorf("root key lookup failed: %v", err)
	}
	if existing == nil {
		return nil // this case should be validated in RPC
	}
	if err := txn.Delete(TableRootKeys, existing); err != nil {
		return fmt.Errorf("root key delete failed: %v", err)
	}

	if err := txn.Insert("index", &IndexEntry{TableRootKeys, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// RootKeys returns an iterator over all root keys
func (s *StateStore) RootKeys(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableRootKeys, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// RootKeyByID returns a specific root key
func (s *StateStore) RootKeyByID(ws memdb.WatchSet, id string) (*structs.RootKey, error) {
	txn := s.db.ReadTxn()

	watchCh, raw, err := txn.FirstWatch(TableRootKeys, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("root key lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if raw != nil {
		return raw.(*structs.RootKey), nil
	}
	return nil, nil
}

// GetActiveRootKey returns the currently active root key
func (s *StateStore) GetActiveRootKey(ws memdb.WatchSet) (*structs.RootKey, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableRootKeys, indexID)
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		key := raw.(*structs.RootKey)
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
