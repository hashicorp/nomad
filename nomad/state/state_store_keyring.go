// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UpsertWrappedRootKeys saves a wrapped root keys or updates them in place.
func (s *StateStore) UpsertWrappedRootKeys(index uint64, wrappedRootKeys *structs.WrappedRootKeys, rekey bool) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// get any existing key for updating
	raw, err := txn.First(TableWrappedRootKeys, indexID, wrappedRootKeys.KeyID)
	if err != nil {
		return fmt.Errorf("root key lookup failed: %v", err)
	}

	isRotation := false

	if raw != nil {
		existing := raw.(*structs.WrappedRootKeys)
		wrappedRootKeys.CreateIndex = existing.CreateIndex
		wrappedRootKeys.CreateTime = existing.CreateTime
		isRotation = !existing.IsActive() && wrappedRootKeys.IsActive()
	} else {
		wrappedRootKeys.CreateIndex = index
		isRotation = wrappedRootKeys.IsActive()
	}
	wrappedRootKeys.ModifyIndex = index

	if rekey && !isRotation {
		return fmt.Errorf("cannot rekey without setting the new key active")
	}

	// if the upsert is for a newly-active key, we need to set all the
	// other keys as inactive in the same transaction.
	if isRotation {
		iter, err := txn.Get(TableWrappedRootKeys, indexID)
		if err != nil {
			return err
		}
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			key := raw.(*structs.WrappedRootKeys)
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
				if err := txn.Insert(TableWrappedRootKeys, key); err != nil {
					return err
				}

			}
		}
	}

	if err := txn.Insert(TableWrappedRootKeys, wrappedRootKeys); err != nil {
		return err
	}
	if err := txn.Insert("index", &IndexEntry{TableWrappedRootKeys, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// DeleteWrappedRootKeys deletes a single wrapped root key set, or returns an
// error if it doesn't exist.
func (s *StateStore) DeleteWrappedRootKeys(index uint64, keyID string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// find the old key
	existing, err := txn.First(TableWrappedRootKeys, indexID, keyID)
	if err != nil {
		return fmt.Errorf("root key lookup failed: %v", err)
	}
	if existing == nil {
		return nil // this case should be validated in RPC
	}
	if err := txn.Delete(TableWrappedRootKeys, existing); err != nil {
		return fmt.Errorf("root key delete failed: %v", err)
	}

	if err := txn.Insert("index", &IndexEntry{TableWrappedRootKeys, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// WrappedRootKeys returns an iterator over all wrapped root keys
func (s *StateStore) WrappedRootKeys(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableWrappedRootKeys, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// WrappedRootKeysByID returns a specific wrapped root key set
func (s *StateStore) WrappedRootKeysByID(ws memdb.WatchSet, id string) (*structs.WrappedRootKeys, error) {
	txn := s.db.ReadTxn()

	watchCh, raw, err := txn.FirstWatch(TableWrappedRootKeys, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("root key lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if raw != nil {
		return raw.(*structs.WrappedRootKeys), nil
	}
	return nil, nil
}

// GetActiveRootKey returns the currently active root key
func (s *StateStore) GetActiveRootKey(ws memdb.WatchSet) (*structs.WrappedRootKeys, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableWrappedRootKeys, indexID)
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		wrappedKeys := raw.(*structs.WrappedRootKeys)
		if wrappedKeys.IsActive() {
			return wrappedKeys, nil
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
