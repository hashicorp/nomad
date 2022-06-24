package state

import (
	"fmt"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

/*
	 _____________________________________________________
	|                                                     |
	|  This contains the ConsulKV Style Secure Variables  |
	|  API. It's split out so I don't lose my mind.       |
	|  -cv                                                |
	|_____________________________________________________|

*/

// SVESet is used to store a secure variable pair.
func (s *StateStore) SVESet(idx uint64, sv *structs.SVERequest) *structs.SVEResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual set.
	resp := svSetTxn(tx, idx, sv)
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
func (s *StateStore) SVESetCAS(idx uint64, sv *structs.SVERequest) *structs.SVEResponse {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	resp := svSetCASTxn(tx, idx, sv)
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
func svSetCASTxn(tx WriteTxn, idx uint64, req *structs.SVERequest) *structs.SVEResponse {
	sv := req.Var
	existing, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed sve lookup: %s", err))
	}
	e, ok := existing.(*structs.SecureVariableEncrypted)
	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if sv.ModifyIndex == 0 && existing != nil {
		return req.ConflictResponse(idx, e)
	}
	if sv.ModifyIndex != 0 && existing == nil {
		return req.ConflictResponse(idx, nil)
	}
	if ok && sv.ModifyIndex != 0 && sv.ModifyIndex != e.ModifyIndex {
		return req.ConflictResponse(idx, e)
	}

	// If we made it this far, we should perform the set.
	return svSetTxn(tx, idx, req)
}

// svSetTxn is used to insert or update a secure variable in the state
// store. It is the inner method used and handles only the actual storage.
func svSetTxn(tx WriteTxn, idx uint64, req *structs.SVERequest) *structs.SVEResponse {
	sv := req.Var
	existingNode, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed sve lookup: %s", err))
	}
	existing, _ := existingNode.(*structs.SecureVariableEncrypted)

	now := time.Now()
	// Set the CreateIndex and CreateTime
	if existing != nil {
		sv.CreateIndex = existing.CreateIndex
		sv.CreateTime = existing.CreateTime
	} else {
		sv.CreateIndex = idx
		sv.CreateTime = now.UnixNano()
	}

	// Set the ModifyIndex.
	if existing != nil && existing.Equals(*sv) {
		// Skip further writing in the state store if the entry is not actually
		// changed. Nevertheless, the input's ModifyIndex should be reset
		// since the TXN API returns a copy in the response.
		sv.ModifyIndex = existing.ModifyIndex
		sv.ModifyTime = existing.ModifyTime
		return nil
	}
	sv.ModifyIndex = idx
	sv.ModifyTime = now.UnixNano()

	// Store the secure variable in the state store and update the index.
	if err := insertSVTxn(tx, sv, false); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed inserting secure variable: %s", err))
	}

	return req.SuccessResponse(idx, &sv.SecureVariableMetadata)
}

func insertSVTxn(tx WriteTxn, sv *structs.SecureVariableEncrypted, updateMax bool) error {
	if err := tx.Insert(TableSecureVariables, sv); err != nil {
		return err
	}
	// updateMax is true during restores and false for normal sets
	if updateMax {
		if err := indexUpdateMaxTxn(tx, sv.ModifyIndex, TableSecureVariables); err != nil {
			return fmt.Errorf("failed updating secure variable index: %v", err)
		}
	} else {
		if err := tx.Insert(tableIndex, &IndexEntry{TableSecureVariables, sv.ModifyIndex}); err != nil {
			return fmt.Errorf("failed updating secure variable index: %s", err)
		}
	}
	return nil
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
func (s *StateStore) SVEDelete(idx uint64, req *structs.SVERequest) *structs.SVEResponse {
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
func (s *StateStore) SVEDeleteCAS(idx uint64, req *structs.SVERequest) *structs.SVEResponse {
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
func (s *StateStore) svDeleteCASTxn(tx WriteTxn, idx uint64, req *structs.SVERequest) *structs.SVEResponse {
	sv := req.Var
	entry, err := tx.First(TableSecureVariables, indexID, sv.Namespace, sv.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed secure variable lookup: %s", err))
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	sve, ok := entry.(*structs.SecureVariableEncrypted)
	if !ok || sv.ModifyIndex != sve.ModifyIndex {
		return req.ConflictResponse(idx, sv)
	}

	// Call the actual deletion if the above passed.
	return s.svDeleteTxn(tx, idx, req)
}

// svDeleteTxn is the inner method used to perform the actual deletion
// of a secure variable within an existing transaction.
func (s *StateStore) svDeleteTxn(tx WriteTxn, idx uint64, req *structs.SVERequest) *structs.SVEResponse {

	// Look up the entry in the state store.
	sv, err := tx.First(TableSecureVariables, indexID, req.Var.Namespace, req.Var.Path)
	if err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed secure variable lookup: %s", err))
	}
	if sv == nil {
		return req.SuccessResponse(idx, nil)
	}

	// TODO: In the Consul code, they create tombstones because there is a risk
	// of the index values going backwards(?). Should we do something similar?

	// // Create a tombstone.
	// if err := s.kvsGraveyard.InsertTxn(tx, key, idx, entMeta); err != nil {
	// 	return fmt.Errorf("failed adding to graveyard: %s", err)
	// }

	return svDeleteWithSVE(tx, idx, req)
}

func svDeleteWithSVE(tx WriteTxn, idx uint64, req *structs.SVERequest) *structs.SVEResponse {
	sv := req.Var
	// Delete the secure variable and update the index table.
	if err := tx.Delete(TableSecureVariables, sv); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed deleting secure variable entry: %s", err))
	}

	if err := tx.Insert(tableIndex, &IndexEntry{TableSecureVariables, idx}); err != nil {
		return req.ErrorResponse(idx, fmt.Errorf("failed updating secure variable index: %s", err))
	}

	return req.SuccessResponse(idx, &sv.SecureVariableMetadata)
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

// indexUpdateMaxTxn is used when restoring entries and sets the table's index to
// the given idx only if it's greater than the current index.
func indexUpdateMaxTxn(tx WriteTxn, idx uint64, table string) error {
	ti, err := tx.First(tableIndex, indexID, table)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing index: %s", err)
	}

	// if this is an update check the idx
	if ti != nil {
		cur, ok := ti.(*IndexEntry)
		if !ok {
			return fmt.Errorf("failed updating index %T need to be `*IndexEntry`", ti)
		}
		// Stored index is newer, don't insert the index
		if idx <= cur.Value {
			return nil
		}
	}

	if err := tx.Insert(tableIndex, &IndexEntry{table, idx}); err != nil {
		return fmt.Errorf("failed updating index %s", err)
	}
	return nil
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
