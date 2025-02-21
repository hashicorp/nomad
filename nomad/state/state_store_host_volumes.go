// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// HostVolumeByID retrieve a specific host volume
func (s *StateStore) HostVolumeByID(ws memdb.WatchSet, ns, id string, withAllocs bool) (*structs.HostVolume, error) {
	txn := s.db.ReadTxn()
	watchCh, obj, err := txn.FirstWatch(TableHostVolumes, indexID, ns, id)
	if err != nil {
		return nil, err
	}
	ws.Add(watchCh)

	if obj == nil {
		return nil, nil
	}
	vol := obj.(*structs.HostVolume)
	if !withAllocs {
		return vol, nil
	}

	vol = vol.Copy()
	vol.Allocations = []*structs.AllocListStub{}

	// we can't use AllocsByNodeTerminal because we only want to filter out
	// allocs that are client-terminal, not server-terminal
	allocs, err := s.AllocsByNode(nil, vol.NodeID)
	if err != nil {
		return nil, fmt.Errorf("could not query allocs to check for host volume claims: %w", err)
	}
	for _, alloc := range allocs {
		if alloc.ClientTerminalStatus() {
			continue
		}
		for _, volClaim := range alloc.Job.LookupTaskGroup(alloc.TaskGroup).Volumes {
			if volClaim.Type == structs.VolumeTypeHost && volClaim.Source == vol.Name {
				vol.Allocations = append(vol.Allocations, alloc.Stub(nil))
			}
		}
	}

	return vol, nil
}

// UpsertHostVolume upserts a host volume
func (s *StateStore) UpsertHostVolume(index uint64, vol *structs.HostVolume) error {
	txn := s.db.WriteTxnMsgT(structs.HostVolumeRegisterRequestType, index)
	defer txn.Abort()

	if exists, err := s.namespaceExists(txn, vol.Namespace); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("host volume %s is in nonexistent namespace %s", vol.ID, vol.Namespace)
	}

	obj, err := txn.First(TableHostVolumes, indexID, vol.Namespace, vol.ID)
	if err != nil {
		return err
	}
	var old *structs.HostVolume
	if obj != nil {
		old = obj.(*structs.HostVolume)
		vol.CreateIndex = old.CreateIndex
		vol.CreateTime = old.CreateTime
	} else {
		vol.CreateIndex = index
	}

	err = s.enforceHostVolumeQuotaTxn(txn, index, vol, old, true)
	if err != nil {
		return err
	}

	// If the fingerprint is written from the node before the create RPC handler
	// completes, we'll never update from the initial pending, so reconcile that
	// here
	node, err := s.NodeByID(nil, vol.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("host volume %s has nonexistent node ID %s", vol.ID, vol.NodeID)
	}

	// prevent a race between node fingerprint and create RPC that could
	// switch a ready volume back to pending
	if _, ok := node.HostVolumes[vol.Name]; ok {
		vol.State = structs.HostVolumeStateReady
	}

	// Register RPCs for new volumes may not have the node pool set
	vol.NodePool = node.NodePool

	// Allocations are denormalized on read, so we don't want these to be
	// written to the state store.
	vol.Allocations = nil
	vol.ModifyIndex = index

	err = txn.Insert(TableHostVolumes, vol)
	if err != nil {
		return fmt.Errorf("host volume insert: %w", err)
	}

	if err := txn.Insert(tableIndex, &IndexEntry{TableHostVolumes, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}

// DeleteHostVolume deletes a host volume
func (s *StateStore) DeleteHostVolume(index uint64, ns string, id string) error {
	txn := s.db.WriteTxnMsgT(structs.HostVolumeDeleteRequestType, index)
	defer txn.Abort()

	obj, err := txn.First(TableHostVolumes, indexID, ns, id)
	if err != nil {
		return err
	}
	if obj != nil {
		vol := obj.(*structs.HostVolume)

		allocs, err := s.AllocsByNodeTerminal(nil, vol.NodeID, false)
		if err != nil {
			return fmt.Errorf("could not query allocs to check for host volume claims: %w", err)
		}
		for _, alloc := range allocs {
			for _, volClaim := range alloc.Job.LookupTaskGroup(alloc.TaskGroup).Volumes {
				if volClaim.Type == structs.VolumeTypeHost && volClaim.Name == vol.Name {
					return fmt.Errorf("could not delete volume %s in use by alloc %s",
						vol.ID, alloc.ID)
				}
			}
		}

		err = s.subtractVolumeFromQuotaUsageTxn(txn, index, vol)
		if err != nil {
			return err
		}

		err = txn.Delete(TableHostVolumes, vol)
		if err != nil {
			return fmt.Errorf("host volume delete: %w", err)
		}
	}

	if err := txn.Insert(tableIndex, &IndexEntry{TableHostVolumes, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()

}

// HostVolumes queries all the host volumes and is mostly used for
// snapshot/restore
func (s *StateStore) HostVolumes(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	return s.hostVolumesIter(ws, indexID, sort)
}

// HostVolumesByIDPrefix retrieves all host volumes by ID prefix. Because the ID
// index is namespaced, we need to handle the wildcard namespace here as well.
func (s *StateStore) HostVolumesByIDPrefix(ws memdb.WatchSet, ns, prefix string, sort SortOption) (memdb.ResultIterator, error) {

	if ns != structs.AllNamespacesSentinel {
		return s.hostVolumesIter(ws, "id_prefix", sort, ns, prefix)
	}

	// for wildcard namespace, wrap the iterator in a filter function that
	// filters all volumes by prefix
	iter, err := s.hostVolumesIter(ws, indexID, sort)
	if err != nil {
		return nil, err
	}
	wrappedIter := memdb.NewFilterIterator(iter, func(raw any) bool {
		vol, ok := raw.(*structs.HostVolume)
		if !ok {
			return true
		}
		return !strings.HasPrefix(vol.ID, prefix)
	})
	return wrappedIter, nil
}

// HostVolumesByName retrieves all host volumes of the same name
func (s *StateStore) HostVolumesByName(ws memdb.WatchSet, ns, name string, sort SortOption) (memdb.ResultIterator, error) {
	return s.hostVolumesIter(ws, "name_prefix", sort, ns, name)
}

// HostVolumesByNodeID retrieves all host volumes on the same node
func (s *StateStore) HostVolumesByNodeID(ws memdb.WatchSet, nodeID string, sort SortOption) (memdb.ResultIterator, error) {
	return s.hostVolumesIter(ws, indexNodeID, sort, nodeID)
}

// HostVolumesByNodePool retrieves all host volumes in the same node pool
func (s *StateStore) HostVolumesByNodePool(ws memdb.WatchSet, nodePool string, sort SortOption) (memdb.ResultIterator, error) {
	return s.hostVolumesIter(ws, indexNodePool, sort, nodePool)
}

func (s *StateStore) hostVolumesIter(ws memdb.WatchSet, index string, sort SortOption, args ...any) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse(TableHostVolumes, index, args...)
	default:
		iter, err = txn.Get(TableHostVolumes, index, args...)
	}
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// upsertHostVolumeForNode sets newly fingerprinted host volumes to ready state
func upsertHostVolumeForNode(txn *txn, node *structs.Node, index uint64) error {
	if len(node.HostVolumes) == 0 {
		return nil
	}
	iter, err := txn.Get(TableHostVolumes, indexNodeID, node.ID)
	if err != nil {
		return err
	}

	var dirty bool // signals we need to update table index

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		vol := raw.(*structs.HostVolume)
		volState := vol.State
		_, ok := node.HostVolumes[vol.Name]

		switch {
		case ok && node.Status == structs.NodeStatusReady &&
			vol.State != structs.HostVolumeStateReady:
			// the fingerprint has been updated on a healthy client
			volState = structs.HostVolumeStateReady

		case !ok && vol.State == structs.HostVolumeStateReady:
			// the volume was previously fingerprinted but is no longer showing
			// up in the fingerprint; this will usually be because of a failed
			// restore on the client
			volState = structs.HostVolumeStateUnavailable

		case ok && node.Status == structs.NodeStatusDown:
			// volumes on down nodes will never pass feasibility checks
			volState = structs.HostVolumeStateUnavailable

		case ok && vol.NodePool != node.NodePool:
			// the client's node pool has been changed

		default:
			// nothing has changed, skip updating this volume
			continue
		}

		vol = vol.Copy()
		vol.State = volState
		vol.NodePool = node.NodePool
		vol.ModifyIndex = index
		err = txn.Insert(TableHostVolumes, vol)
		if err != nil {
			return fmt.Errorf("host volume insert: %w", err)
		}
		dirty = true
	}

	if dirty {
		if err := txn.Insert("index", &IndexEntry{TableHostVolumes, index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	return nil
}
