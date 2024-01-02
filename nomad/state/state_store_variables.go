// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"math"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Variables queries all the variables and is used only for
// snapshot/restore and key rotation
func (s *StateStore) Variables(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableVariables, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// GetVariablesByNamespace returns an iterator that contains all
// variables belonging to the provided namespace.
func (s *StateStore) GetVariablesByNamespace(
	ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	return s.getVariablesByNamespaceImpl(txn, ws, namespace)
}

func (s *StateStore) getVariablesByNamespaceImpl(
	txn *txn, ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	// Walk the entire table.
	iter, err := txn.Get(TableVariables, indexID+"_prefix", namespace, "")
	if err != nil {
		return nil, fmt.Errorf("variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetVariablesByNamespaceAndPrefix returns an iterator that contains all
// variables belonging to the provided namespace that match the prefix.
func (s *StateStore) GetVariablesByNamespaceAndPrefix(
	ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableVariables, indexID+"_prefix", namespace, prefix)
	if err != nil {
		return nil, fmt.Errorf("variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetVariablesByPrefix returns an iterator that contains all variables that
// match the prefix in any namespace. Namespace filtering is the responsibility
// of the caller.
func (s *StateStore) GetVariablesByPrefix(
	ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table.
	iter, err := txn.Get(TableVariables, indexPath+"_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetVariablesByKeyID returns an iterator that contains all
// variables that were encrypted with a particular key
func (s *StateStore) GetVariablesByKeyID(
	ws memdb.WatchSet, keyID string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableVariables, indexKeyID, keyID)
	if err != nil {
		return nil, fmt.Errorf("variable lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// GetVariable returns a single variable at a given namespace and
// path.
func (s *StateStore) GetVariable(
	ws memdb.WatchSet, namespace, path string) (*structs.VariableEncrypted, error) {
	txn := s.db.ReadTxn()

	// Try to fetch the variable.
	watchCh, raw, err := txn.FirstWatch(TableVariables, indexID, namespace, path)
	if err != nil { // error during fetch
		return nil, fmt.Errorf("variable lookup failed: %v", err)
	}
	ws.Add(watchCh)
	if raw == nil { // not found
		return nil, nil
	}

	sv := raw.(*structs.VariableEncrypted)
	return sv, nil
}

// VarSet is used to store a variable object.
func (s *StateStore) VarSet(idx uint64, sv *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual set.
	resp := s.varSetTxn(tx, idx, sv)
	if resp.IsError() {
		return resp
	}

	if err := tx.Commit(); err != nil {
		return sv.ErrorResponse(idx, err)
	}
	return resp
}

// VarSetCAS is used to do a check-and-set operation on a
// variable. The ModifyIndex in the provided entry is used to determine if
// we should write the entry to the state store or not.
func (s *StateStore) VarSetCAS(idx uint64, sv *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	resp := s.varSetCASTxn(tx, idx, sv)
	if resp.IsError() || resp.IsConflict() {
		return resp
	}

	if err := tx.Commit(); err != nil {
		return sv.ErrorResponse(idx, err)
	}
	return resp
}

// varSetCASTxn is the inner method used to do a CAS inside an existing
// transaction.
func (s *StateStore) varSetCASTxn(tx WriteTxn, idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
	sv := req.Var
	raw, err := tx.First(TableVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed variable lookup: %s", err))
	}
	svEx, ok := raw.(*structs.VariableEncrypted)

	// ModifyIndex of 0 means that we are doing a set-if-not-exists.
	if sv.ModifyIndex == 0 && raw != nil {
		return req.ConflictResponse(idx, svEx)
	}

	// If the ModifyIndex is set but the variable doesn't exist, return a
	// plausible zero value as the conflict
	if sv.ModifyIndex != 0 && raw == nil {
		zeroVal := &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
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
	return s.varSetTxn(tx, idx, req)
}

// varSetTxn is used to insert or update a variable in the state
// store. It is the inner method used and handles only the actual storage.
func (s *StateStore) varSetTxn(tx WriteTxn, idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
	sv := req.Var
	existingRaw, err := tx.First(TableVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed sve lookup: %s", err))
	}
	existing, _ := existingRaw.(*structs.VariableEncrypted)

	existingQuota, err := tx.First(TableVariablesQuotas, indexID, sv.Namespace)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("variable quota lookup failed: %v", err))
	}

	var quotaChange int64

	// Set the CreateIndex and CreateTime
	if existing != nil {
		sv.CreateIndex = existing.CreateIndex
		sv.CreateTime = existing.CreateTime

		if existing.Equal(*sv) {
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

	if err := tx.Insert(TableVariables, sv); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed inserting variable: %s", err))
	}

	// Track quota usage
	var quotaUsed *structs.VariablesQuota
	if existingQuota != nil {
		quotaUsed = existingQuota.(*structs.VariablesQuota)
		quotaUsed = quotaUsed.Copy()
	} else {
		quotaUsed = &structs.VariablesQuota{
			Namespace:   sv.Namespace,
			CreateIndex: idx,
		}
	}

	if quotaChange > math.MaxInt64-quotaUsed.Size {
		// this limit is actually shared across all namespaces in the region's
		// quota (if there is one), but we need this check here to prevent
		// overflow as well
		return req.ErrorResponse(idx, fmt.Errorf("variables can store a maximum of %d bytes of encrypted data per namespace", math.MaxInt))
	}

	if quotaChange > 0 {
		quotaUsed.Size += quotaChange
	} else if quotaChange < 0 {
		quotaUsed.Size -= min(quotaUsed.Size, -quotaChange)
	}

	err = s.enforceVariablesQuota(idx, tx, sv.Namespace, quotaChange)
	if err != nil {
		return req.ErrorResponse(idx, err)
	}

	// we check enforcement above even if there's no change because another
	// namespace may have used up quota to make this no longer valid, but we
	// only update the table if this namespace has changed
	if quotaChange != 0 {
		quotaUsed.ModifyIndex = idx
		if err := tx.Insert(TableVariablesQuotas, quotaUsed); err != nil {
			return req.ErrorResponse(idx, fmt.Errorf("variable quota insert failed: %v", err))
		}
	}

	if err := tx.Insert(tableIndex,
		&IndexEntry{TableVariables, idx}); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed updating variable index: %s", err))
	}

	return req.SuccessResponse(idx, &sv.VariableMetadata)
}

// VarDelete is used to delete a single variable in the
// the state store.
func (s *StateStore) VarDelete(idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
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

// VarDeleteCAS is used to conditionally delete a variable if and only if it has
// a given modify index. If the CAS index (cidx) specified is not equal to the
// last observed index for the given variable, then the call is a noop,
// otherwise a normal delete is invoked.
func (s *StateStore) VarDeleteCAS(idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
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
// of a variable within an existing transaction as part of a
// conditional delete.
func (s *StateStore) svDeleteCASTxn(tx WriteTxn, idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {
	sv := req.Var
	raw, err := tx.First(TableVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed variable lookup: %s", err))
	}

	// ModifyIndex of 0 means that we are doing a delete-if-not-exists, so when
	// raw == nil, it is successful. We should return here without manipulating
	// the state store further.
	if sv.ModifyIndex == 0 && raw == nil {
		return req.SuccessResponse(idx, nil)
	}

	// If the ModifyIndex is set but the variable doesn't exist, return a
	// plausible zero value as the conflict, because the user _expected_ there
	// to have been a value and its absence is a conflict.
	if sv.ModifyIndex != 0 && raw == nil {
		zeroVal := &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Namespace: sv.Namespace,
				Path:      sv.Path,
			},
		}
		return req.ConflictResponse(idx, zeroVal)
	}

	// Any work beyond this point needs to be able to consult the actual
	// returned content, so assert it back into the right type.
	svEx, ok := raw.(*structs.VariableEncrypted)

	// ModifyIndex of 0 means that we are doing a delete-if-not-exists, but
	// there was a value stored in the state store
	if sv.ModifyIndex == 0 && raw != nil {
		return req.ConflictResponse(idx, svEx)
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
// of a variable within an existing transaction.
func (s *StateStore) svDeleteTxn(tx WriteTxn, idx uint64, req *structs.VarApplyStateRequest) *structs.VarApplyStateResponse {

	// Look up the entry in the state store.
	existingRaw, err := tx.First(TableVariables, indexID, req.Var.Namespace, req.Var.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed variable lookup: %s", err))
	}
	if existingRaw == nil {
		return req.SuccessResponse(idx, nil)
	}

	existingQuota, err := tx.First(TableVariablesQuotas, indexID, req.Var.Namespace)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("variable quota lookup failed: %v", err))
	}

	sv := existingRaw.(*structs.VariableEncrypted)

	// Track quota usage
	if existingQuota != nil {
		quotaUsed := existingQuota.(*structs.VariablesQuota)
		quotaUsed = quotaUsed.Copy()
		quotaUsed.Size -= min(quotaUsed.Size, int64(len(sv.Data)))
		quotaUsed.ModifyIndex = idx
		if err := tx.Insert(TableVariablesQuotas, quotaUsed); err != nil {
			return req.ErrorResponse(idx, fmt.Errorf("variable quota insert failed: %v", err))
		}
	}

	// Delete the variable and update the index table.
	if err := tx.Delete(TableVariables, sv); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed deleting variable entry: %s", err))
	}

	if err := tx.Insert(tableIndex, &IndexEntry{TableVariables, idx}); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed updating variable index: %s", err))
	}

	return req.SuccessResponse(idx, nil)
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

// VariablesQuotas queries all the quotas and is used only for
// snapshot/restore and key rotation
func (s *StateStore) VariablesQuotas(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableVariablesQuotas, indexID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// VariablesQuotaByNamespace queries for quotas for a particular namespace
func (s *StateStore) VariablesQuotaByNamespace(ws memdb.WatchSet, namespace string) (*structs.VariablesQuota, error) {
	txn := s.db.ReadTxn()
	watchCh, raw, err := txn.FirstWatch(TableVariablesQuotas, indexID, namespace)
	if err != nil {
		return nil, fmt.Errorf("variable quota lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if raw == nil {
		return nil, nil
	}
	quotaUsed := raw.(*structs.VariablesQuota)
	return quotaUsed, nil
}
