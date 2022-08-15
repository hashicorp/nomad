package state

import (
	"fmt"
	"math"

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

// SVESet is used to store a secure variable pair.
func (s *StateStore) SVESet(idx uint64, sv *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual set.
	resp := s.svSetTxn(tx, idx, sv)
	if resp.IsError() {
		return resp
	}

	if err := tx.Commit(); err != nil {
		return sv.ErrorResponse(idx, err)
	}
	return resp
}

// SVESetCAS is used to do a check-and-set operation on a secure
// variable. The ModifyIndex in the provided entry is used to determine if
// we should write the entry to the state store or not. Returns a bool
// indicating whether or not a write happened and any error that occurred.
func (s *StateStore) SVESetCAS(idx uint64, sv *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	resp := s.svSetCASTxn(tx, idx, sv)
	if resp.IsError() || resp.IsConflict() {
		return resp
	}

	if err := tx.Commit(); err != nil {
		return sv.ErrorResponse(idx, err)
	}
	return resp
}

// svSetCASTxn is the inner method used to do a CAS inside an existing
// transaction.
func (s *StateStore) svSetCASTxn(tx WriteTxn, idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	sv := req.Var
	raw, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed sve lookup: %s", err))
	}
	svEx, ok := raw.(*structs.SecureVariableEncrypted)

	// ModifyIndex of 0 means that we are doing a set-if-not-exists.
	if sv.ModifyIndex == 0 && raw != nil {
		return req.ConflictResponse(idx, svEx)
	}

	// If the ModifyIndex is set but the variable doesn't exist, return a
	// plausible zero value as the conflict
	if sv.ModifyIndex != 0 && raw == nil {
		zeroVal := &structs.SecureVariableEncrypted{
			SecureVariableMetadata: structs.SecureVariableMetadata{
				Namespace: sv.Namespace,
				Path:      sv.Path,
			},
		}
		return req.ConflictResponse(idx, zeroVal)
	}

	// If the existing index does not match the provided CAS index arg, then we
	// shouldn't update anything and can safely return early here.
	if ok && sv.ModifyIndex != svEx.ModifyIndex {
		return req.ConflictResponse(idx, svEx)
	}

	// If we made it this far, we should perform the set.
	return s.svSetTxn(tx, idx, req)
}

// svSetTxn is used to insert or update a secure variable in the state
// store. It is the inner method used and handles only the actual storage.
func (s *StateStore) svSetTxn(tx WriteTxn, idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	sv := req.Var
	existingRaw, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed sve lookup: %s", err))
	}
	existing, _ := existingRaw.(*structs.SecureVariableEncrypted)

	existingQuota, err := tx.First(TableSecureVariablesQuotas, indexID, sv.Namespace)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("secure variable quota lookup failed: %v", err))
	}

	var quotaChange int64

	// Set the CreateIndex and CreateTime
	if existing != nil {
		sv.CreateIndex = existing.CreateIndex
		sv.CreateTime = existing.CreateTime

		if existing.Equals(*sv) {
			// Skip further writing in the state store if the entry is not actually
			// changed. Nevertheless, the input's ModifyIndex should be reset
			// since the TXN API returns a copy in the response.
			sv.ModifyIndex = existing.ModifyIndex
			sv.ModifyTime = existing.ModifyTime
			return req.SuccessResponse(idx, nil)
		}
		sv.ModifyIndex = idx
		quotaChange = int64(len(sv.Data) - len(existing.Data))
	} else {
		sv.CreateIndex = idx
		sv.ModifyIndex = idx
		quotaChange = int64(len(sv.Data))
	}

	if err := tx.Insert(TableSecureVariables, sv); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed inserting secure variable: %s", err))
	}

	// Track quota usage
	var quotaUsed *structs.SecureVariablesQuota
	if existingQuota != nil {
		quotaUsed = existingQuota.(*structs.SecureVariablesQuota)
		quotaUsed = quotaUsed.Copy()
	} else {
		quotaUsed = &structs.SecureVariablesQuota{
			Namespace:   sv.Namespace,
			CreateIndex: idx,
		}
	}

	if quotaChange > math.MaxInt64-quotaUsed.Size {
		// this limit is actually shared across all namespaces in the region's
		// quota (if there is one), but we need this check here to prevent
		// overflow as well
		return req.ErrorResponse(idx, fmt.Errorf("secure variables can store a maximum of %d bytes of encrypted data per namespace", math.MaxInt))
	}

	if quotaChange > 0 {
		quotaUsed.Size += quotaChange
	} else if quotaChange < 0 {
		quotaUsed.Size -= helper.Min(quotaUsed.Size, -quotaChange)
	}

	err = s.enforceSecureVariablesQuota(idx, tx, sv.Namespace, quotaChange)
	if err != nil {
		return req.ErrorResponse(idx, err)
	}

	// we check enforcement above even if there's no change because another
	// namespace may have used up quota to make this no longer valid, but we
	// only update the table if this namespace has changed
	if quotaChange != 0 {
		quotaUsed.ModifyIndex = idx
		if err := tx.Insert(TableSecureVariablesQuotas, quotaUsed); err != nil {
			return req.ErrorResponse(idx, fmt.Errorf("secure variable quota insert failed: %v", err))
		}
	}

	if err := tx.Insert(tableIndex,
		&IndexEntry{TableSecureVariables, idx}); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed updating secure variable index: %s", err))
	}

	return req.SuccessResponse(idx, &sv.SecureVariableMetadata)
}

// SVEGet is used to retrieve a key/value pair from the state store.
func (s *StateStore) SVEGet(ws memdb.WatchSet, namespace, path string) (uint64, *structs.SecureVariableEncrypted, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return svGetTxn(tx, ws, namespace, path)
}

// svGetTxn is the inner method that gets a secure variable inside an existing
// transaction.
func svGetTxn(tx ReadTxn,
	ws memdb.WatchSet, namespace, path string) (uint64, *structs.SecureVariableEncrypted, error) {

	// Get the table index.
	idx := svMaxIndex(tx)

	watchCh, entry, err := tx.FirstWatch(TableSecureVariables, indexID, namespace, path)
	if err != nil {
		return 0, nil, fmt.Errorf("failed secure variable lookup: %s", err)
	}
	ws.Add(watchCh)
	if entry != nil {
		return idx, entry.(*structs.SecureVariableEncrypted), nil
	}
	return idx, nil, nil
}

// SVEDelete is used to delete a single secure variable in the
// the state store.
func (s *StateStore) SVEDelete(idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual delete
	resp := s.svDeleteTxn(tx, idx, req)
	if !resp.IsOk() {
		return resp
	}

	err := tx.Commit()
	if err != nil {
		return req.ErrorResponse(idx, err)
	}

	return resp
}

// SVEDeleteCAS is used to conditionally delete a secure
// variable if and only if it has a given modify index. If the CAS
// index (cidx) specified is not equal to the last observed index for
// the given variable, then the call is a noop, otherwise a normal
// delete is invoked.
func (s *StateStore) SVEDeleteCAS(idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	resp := s.svDeleteCASTxn(tx, idx, req)
	if !resp.IsOk() {
		return resp
	}

	err := tx.Commit()
	if err != nil {
		return req.ErrorResponse(idx, err)
	}

	return resp
}

// svDeleteCASTxn is an inner method used to check the existing value
// of a secure variable within an existing transaction as part of a
// conditional delete.
func (s *StateStore) svDeleteCASTxn(tx WriteTxn, idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {
	sv := req.Var
	raw, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed secure variable lookup: %s", err))
	}

	svEx, ok := raw.(*structs.SecureVariableEncrypted)

	// ModifyIndex of 0 means that we are doing a delete-if-not-exists.
	if sv.ModifyIndex == 0 && raw != nil {
		return req.ConflictResponse(idx, svEx)
	}

	// If the ModifyIndex is set but the variable doesn't exist, return a
	// plausible zero value as the conflict
	if sv.ModifyIndex != 0 && raw == nil {
		zeroVal := &structs.SecureVariableEncrypted{
			SecureVariableMetadata: structs.SecureVariableMetadata{
				Namespace: sv.Namespace,
				Path:      sv.Path,
			},
		}
		return req.ConflictResponse(idx, zeroVal)
	}

	// If the existing index does not match the provided CAS index arg, then we
	// shouldn't update anything and can safely return early here.
	if !ok || sv.ModifyIndex != svEx.ModifyIndex {
		return req.ConflictResponse(idx, svEx)
	}

	// Call the actual deletion if the above passed.
	return s.svDeleteTxn(tx, idx, req)
}

// svDeleteTxn is the inner method used to perform the actual deletion
// of a secure variable within an existing transaction.
func (s *StateStore) svDeleteTxn(tx WriteTxn, idx uint64, req *structs.SVApplyStateRequest) *structs.SVApplyStateResponse {

	// Look up the entry in the state store.
	existingRaw, err := tx.First(TableSecureVariables, indexID, req.Var.Namespace, req.Var.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed secure variable lookup: %s", err))
	}
	if existingRaw == nil {
		return req.SuccessResponse(idx, nil)
	}

	existingQuota, err := tx.First(TableSecureVariablesQuotas, indexID, req.Var.Namespace)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("secure variable quota lookup failed: %v", err))
	}

	sv := existingRaw.(*structs.SecureVariableEncrypted)

	// Track quota usage
	if existingQuota != nil {
		quotaUsed := existingQuota.(*structs.SecureVariablesQuota)
		quotaUsed = quotaUsed.Copy()
		quotaUsed.Size -= helper.Min(quotaUsed.Size, int64(len(sv.Data)))
		quotaUsed.ModifyIndex = idx
		if err := tx.Insert(TableSecureVariablesQuotas, quotaUsed); err != nil {
			return req.ErrorResponse(idx, fmt.Errorf("secure variable quota insert failed: %v", err))
		}
	}

	// Delete the secure variable and update the index table.
	if err := tx.Delete(TableSecureVariables, sv); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed deleting secure variable entry: %s", err))
	}

	if err := tx.Insert(tableIndex, &IndexEntry{TableSecureVariables, idx}); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed updating secure variable index: %s", err))
	}

	return req.SuccessResponse(idx, nil)
}

// This extra indirection is to facilitate the tombstone case if it matters.
func svMaxIndex(tx ReadTxn) uint64 {
	return maxIndexTxn(tx, TableSecureVariables)
}

// WriteTxn is implemented by memdb.Txn to perform write operations.
type WriteTxn interface {
	ReadTxn
	Defer(func())
	Delete(table string, obj interface{}) error
	DeleteAll(table, index string, args ...interface{}) (int, error)
	DeletePrefix(table string, index string, prefix string) (bool, error)
	Insert(table string, obj interface{}) error
}

// maxIndex is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func (s *StateStore) maxIndex(tables ...string) uint64 {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return maxIndexTxn(tx, tables...)
}

// maxIndexTxn is a helper used to retrieve the highest known index
// amongst a set of tables in the db.
func maxIndexTxn(tx ReadTxn, tables ...string) uint64 {
	return maxIndexWatchTxn(tx, nil, tables...)
}

func maxIndexWatchTxn(tx ReadTxn, ws memdb.WatchSet, tables ...string) uint64 {
	var lindex uint64
	for _, table := range tables {
		ch, ti, err := tx.FirstWatch(tableIndex, "id", table)
		if err != nil {
			panic(fmt.Sprintf("unknown index: %s err: %s", table, err))
		}
		if idx, ok := ti.(*IndexEntry); ok && idx.Value > lindex {
			lindex = idx.Value
		}
		ws.Add(ch)
	}
	return lindex
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
