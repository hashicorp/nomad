// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// nodePoolInit creates the built-in node pools that should always be present
// in the cluster.
func (s *StateStore) nodePoolInit() error {
	allNodePool := &structs.NodePool{
		Name:        structs.NodePoolAll,
		Description: structs.NodePoolAllDescription,
	}

	defaultNodePool := &structs.NodePool{
		Name:        structs.NodePoolDefault,
		Description: structs.NodePoolDefaultDescription,
	}

	return s.UpsertNodePools(
		structs.NodePoolUpsertRequestType,
		1,
		[]*structs.NodePool{allNodePool, defaultNodePool},
	)
}

// NodePools returns an iterator over all node pools.
func (s *StateStore) NodePools(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableNodePools, "id")
	if err != nil {
		return nil, fmt.Errorf("node pools lookup failed: %w", err)
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// NodePoolByName returns the node pool that matches the given name or nil if
// there is no match.
func (s *StateStore) NodePoolByName(ws memdb.WatchSet, name string) (*structs.NodePool, error) {
	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch(TableNodePools, "id", name)
	if err != nil {
		return nil, fmt.Errorf("node pool lookup failed: %w", err)
	}
	ws.Add(watchCh)

	if existing == nil {
		return nil, nil
	}

	return existing.(*structs.NodePool), nil
}

// NodePoolsByNamePrefix returns an interator over all node pools that match
// the given name prefix.
func (s *StateStore) NodePoolsByNamePrefix(ws memdb.WatchSet, namePrefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableNodePools, "id_prefix", namePrefix)
	if err != nil {
		return nil, fmt.Errorf("node pool lookup failed: %w", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertNodePools inserts or updates the given set of node pools.
func (s *StateStore) UpsertNodePools(msgType structs.MessageType, index uint64, pools []*structs.NodePool) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	for _, pool := range pools {
		if err := s.upsertNodePoolTxn(txn, index, pool); err != nil {
			return err
		}
	}

	if err := txn.Insert("index", &IndexEntry{TableNodePools, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}

func (s *StateStore) upsertNodePoolTxn(txn *txn, index uint64, pool *structs.NodePool) error {
	if pool == nil {
		return nil
	}

	existing, err := txn.First(TableNodePools, "id", pool.Name)
	if err != nil {
		return fmt.Errorf("node pool lookup failed: %w", err)
	}

	if existing != nil {
		// Prevent changes to built-in node pools.
		if pool.IsBuiltIn() {
			return fmt.Errorf("modifying node pool %q is not allowed", pool.Name)
		}

		exist := existing.(*structs.NodePool)
		pool.CreateIndex = exist.CreateIndex
		pool.ModifyIndex = index
	} else {
		pool.CreateIndex = index
		pool.ModifyIndex = index
	}

	if err := txn.Insert(TableNodePools, pool); err != nil {
		return fmt.Errorf("node pool insert failed: %w", err)
	}

	return nil
}

// DeleteNodePools removes the given set of node pools.
func (s *StateStore) DeleteNodePools(msgType structs.MessageType, index uint64, names []string) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	for _, n := range names {
		if err := s.deleteNodePoolTxn(txn, index, n); err != nil {
			return err
		}
	}

	// Update index table.
	if err := txn.Insert("index", &IndexEntry{TableNodePools, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}

func (s *StateStore) deleteNodePoolTxn(txn *txn, index uint64, name string) error {
	// Check if node pool exists.
	existing, err := txn.First(TableNodePools, "id", name)
	if err != nil {
		return fmt.Errorf("node pool lookup failed: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("node pool %s not found", name)
	}

	pool := existing.(*structs.NodePool)

	// Prevent deletion of built-in node pools.
	if pool.IsBuiltIn() {
		return fmt.Errorf("deleting node pool %q is not allowed", pool.Name)
	}

	// Delete node pool.
	if err := txn.Delete(TableNodePools, pool); err != nil {
		return fmt.Errorf("node pool deletion failed: %w", err)
	}

	return nil
}
