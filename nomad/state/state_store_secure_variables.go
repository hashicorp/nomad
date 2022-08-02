package state

import (
	"fmt"
	"math"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SecureVariables queries all the variables and is used only for
// snapshot/restore and key rotation
func (s *StateStore) SecureVariables(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableSecureVariables, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// GetSecureVariablesByNamespace returns an iterator that contains all
// variables belonging to the provided namespace.
func (s *StateStore) GetSecureVariablesByNamespace(
	ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	return s.getSecureVariablesByNamespaceImpl(txn, ws, namespace)
}

func (s *StateStore) getSecureVariablesByNamespaceImpl(
	txn *txn, ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	// Walk the entire table.
	iter, err := txn.Get(TableSecureVariables, indexID+"_prefix", namespace, "")
	if err != nil {
		return nil, fmt.Errorf("secure variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetSecureVariablesByNamespaceAndPrefix returns an iterator that contains all
// variables belonging to the provided namespace that match the prefix.
func (s *StateStore) GetSecureVariablesByNamespaceAndPrefix(
	ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableSecureVariables, indexID+"_prefix", namespace, prefix)
	if err != nil {
		return nil, fmt.Errorf("secure variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetSecureVariablesByPrefix returns an iterator that contains all variables that
// match the prefix in any namespace. Namespace filtering is the responsibility
// of the caller.
func (s *StateStore) GetSecureVariablesByPrefix(
	ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableSecureVariables, indexPath+"_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("secure variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetSecureVariablesByKeyID returns an iterator that contains all
// variables that were encrypted with a particular key
func (s *StateStore) GetSecureVariablesByKeyID(
	ws memdb.WatchSet, keyID string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableSecureVariables, indexKeyID, keyID)
	if err != nil {
		return nil, fmt.Errorf("secure variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetSecureVariable returns a single secure variable at a given namespace and
// path.
func (s *StateStore) GetSecureVariable(
	ws memdb.WatchSet, namespace, path string) (*structs.SecureVariableEncrypted, error) {
	txn := s.db.ReadTxn()

	// Try to fetch the secure variable.
	raw, err := txn.First(TableSecureVariables, indexID, namespace, path)
	if err != nil { // error during fetch
		return nil, fmt.Errorf("secure variable lookup failed: %v", err)
	}
	if raw == nil { // not found
		return nil, nil
	}

	sv := raw.(*structs.SecureVariableEncrypted)
	return sv, nil
}

func (s *StateStore) UpsertSecureVariables(msgType structs.MessageType, index uint64, svs []*structs.SecureVariableEncrypted) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	var updated bool = false
	for _, sv := range svs {
		if err := s.upsertSecureVariableImpl(index, txn, sv, &updated); err != nil {
			return err
		}
	}

	if !updated {
		return nil
	}

	if err := txn.Insert(tableIndex, &IndexEntry{TableSecureVariables, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertSecureVariableImpl is used to upsert a secure variable
func (s *StateStore) upsertSecureVariableImpl(index uint64, txn *txn, sv *structs.SecureVariableEncrypted, updated *bool) error {
	// Check if the secure variable already exists
	existing, err := txn.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return fmt.Errorf("secure variable lookup failed: %v", err)
	}

	existingQuota, err := txn.First(TableSecureVariablesQuotas, indexID, sv.Namespace)
	if err != nil {
		return fmt.Errorf("secure variable quota lookup failed: %v", err)
	}

	var quotaChange int64

	// Setup the indexes correctly
	nowNano := time.Now().UnixNano()
	if existing != nil {
		exist := existing.(*structs.SecureVariableEncrypted)
		if !shouldWrite(sv, exist) {
			*updated = false
			return nil
		}
		sv.CreateIndex = exist.CreateIndex
		sv.CreateTime = exist.CreateTime
		sv.ModifyIndex = index
		sv.ModifyTime = nowNano
		quotaChange = int64(len(sv.Data) - len(exist.Data))
	} else {
		sv.CreateIndex = index
		sv.CreateTime = nowNano
		sv.ModifyIndex = index
		sv.ModifyTime = nowNano
		quotaChange = int64(len(sv.Data))
	}

	// Insert the secure variable
	if err := txn.Insert(TableSecureVariables, sv); err != nil {
		return fmt.Errorf("secure variable insert failed: %v", err)
	}

	// Track quota usage
	var quotaUsed *structs.SecureVariablesQuota
	if existingQuota != nil {
		quotaUsed = existingQuota.(*structs.SecureVariablesQuota)
		quotaUsed = quotaUsed.Copy()
	} else {
		quotaUsed = &structs.SecureVariablesQuota{
			Namespace:   sv.Namespace,
			CreateIndex: index,
		}
	}

	if quotaChange > math.MaxInt64-quotaUsed.Size {
		// this limit is actually shared across all namespaces in the region's
		// quota (if there is one), but we need this check here to prevent
		// overflow as well
		return fmt.Errorf("secure variables can store a maximum of %d bytes of encrypted data per namespace", math.MaxInt)
	}

	if quotaChange > 0 {
		quotaUsed.Size += quotaChange
	} else if quotaChange < 0 {
		quotaUsed.Size -= helper.Min(quotaUsed.Size, -quotaChange)
	}

	err = s.enforceSecureVariablesQuota(index, txn, sv.Namespace, quotaChange)
	if err != nil {
		return err
	}

	// we check enforcement above even if there's no change because another
	// namespace may have used up quota to make this no longer valid, but we
	// only update the table if this namespace has changed
	if quotaChange != 0 {
		quotaUsed.ModifyIndex = index
		if err := txn.Insert(TableSecureVariablesQuotas, quotaUsed); err != nil {
			return fmt.Errorf("secure variable quota insert failed: %v", err)
		}
	}

	*updated = true
	return nil
}

// shouldWrite can be used to determine if a write needs to happen.
func shouldWrite(sv, existing *structs.SecureVariableEncrypted) bool {
	// FIXME: Move this to the RPC layer eventually.
	if existing == nil {
		return true
	}
	if sv.Equals(*existing) {
		return false
	}
	return true
}

func (s *StateStore) DeleteSecureVariables(msgType structs.MessageType, index uint64, namespace string, paths []string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeleteSecureVariablesTxn(index, namespace, paths, txn)
	if err == nil {
		return txn.Commit()
	}
	return err
}

func (s *StateStore) DeleteSecureVariablesTxn(index uint64, namespace string, paths []string, txn Txn) error {
	for _, path := range paths {
		err := s.DeleteSecureVariableTxn(index, namespace, path, txn)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteSecureVariable is used to delete a single secure variable
func (s *StateStore) DeleteSecureVariable(index uint64, namespace, path string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeleteSecureVariableTxn(index, namespace, path, txn)
	if err == nil {
		return txn.Commit()
	}
	return err
}

// DeleteSecureVariableTxn is used to delete the secure variable, like DeleteSecureVariable
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) DeleteSecureVariableTxn(index uint64, namespace, path string, txn Txn) error {
	// Lookup the variable
	existing, err := txn.First(TableSecureVariables, indexID, namespace, path)
	if err != nil {
		return fmt.Errorf("secure variable lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("secure variable not found")
	}
	existingQuota, err := txn.First(TableSecureVariablesQuotas, indexID, namespace)
	if err != nil {
		return fmt.Errorf("secure variable quota lookup failed: %v", err)
	}

	// Delete the variable
	if err := txn.Delete(TableSecureVariables, existing); err != nil {
		return fmt.Errorf("secure variable delete failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{TableSecureVariables, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Track quota usage
	if existingQuota != nil {
		quotaUsed := existingQuota.(*structs.SecureVariablesQuota)
		quotaUsed = quotaUsed.Copy()
		sv := existing.(*structs.SecureVariableEncrypted)
		quotaUsed.Size -= helper.Min(quotaUsed.Size, int64(len(sv.Data)))
		quotaUsed.ModifyIndex = index
		if err := txn.Insert(TableSecureVariablesQuotas, quotaUsed); err != nil {
			return fmt.Errorf("secure variable quota insert failed: %v", err)
		}
	}

	return nil
}

// SecureVariablesQuotas queries all the quotas and is used only for
// snapshot/restore and key rotation
func (s *StateStore) SecureVariablesQuotas(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableSecureVariablesQuotas, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// SecureVariablesQuotaByNamespace queries for quotas for a particular namespace
func (s *StateStore) SecureVariablesQuotaByNamespace(ws memdb.WatchSet, namespace string) (*structs.SecureVariablesQuota, error) {
	txn := s.db.ReadTxn()
	watchCh, raw, err := txn.FirstWatch(TableSecureVariablesQuotas, indexID, namespace)
	if err != nil {
		return nil, fmt.Errorf("secure variable quota lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if raw == nil {
		return nil, nil
	}
	quotaUsed := raw.(*structs.SecureVariablesQuota)
	return quotaUsed, nil
}
