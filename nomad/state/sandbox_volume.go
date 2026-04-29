// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *StateStore) SandboxVolumeByID(ws memdb.WatchSet, ns, id string) (*structs.SandboxVolume, error) {
	txn := s.db.ReadTxn()
	watchCh, obj, err := txn.FirstWatch(TableSandboxVolumes, indexID, ns, id)
	if err != nil {
		return nil, err
	}
	ws.Add(watchCh)
	if obj == nil {
		return nil, nil
	}
	return obj.(*structs.SandboxVolume), nil
}

func (s *StateStore) SandboxesByName(ws memdb.WatchSet, ns, name string, forMode structs.VolumeAccessMode) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	iter, err = txn.Get(TableSandboxVolumes, indexName, ns, name)
	if err != nil {
		return nil, err
	}
	// switch forMode {
	// case structs.HostVolumeAccessModeSingleNodeSingleWriter:
	// 	iter = memdb.NewFilterIterator(iter, func(obj any) bool {
	// 		sandbox := obj.(*structs.SandboxVolume)
	// 		return len(sandbox.AllocIDs) == 0
	// 	})
	// default:
	// 	// TODO: check r/o?
	// }

	ws.Add(iter.WatchCh())
	return iter, nil
}

// claimSandboxVolumes is called whenever we update an allocation
func (s *StateStore) claimSandboxVolumes(txn Txn, index uint64, alloc *structs.Allocation) error {
	if len(alloc.AllocatedResources.Shared.Sandboxes) == 0 {
		return nil
	}

	now := alloc.ModifyTime

	for _, allocatedSandbox := range alloc.AllocatedResources.Shared.Sandboxes {
		obj, err := txn.First(TableSandboxVolumes, indexID,
			alloc.Namespace, allocatedSandbox.ID)
		if err != nil {
			return err
		}
		if obj != nil {
			old := obj.(*structs.SandboxVolume)
			err := s.updateSandboxVolumeClaim(txn, index, now, old, alloc)
			if err != nil {
				// ex. may be already claimed for exclusive access
				return fmt.Errorf("could not claim sandbox: %w", err)
			}
			continue
		}

		// create a new sandbox and claim it for the allocation
		vol := &structs.SandboxVolume{
			ID:            allocatedSandbox.ID,
			Namespace:     alloc.Namespace,
			Name:          allocatedSandbox.Name,
			AllocIDs:      []string{alloc.ID},
			NodeID:        alloc.NodeID,
			CapacityBytes: allocatedSandbox.CapacityBytes,
			TTL:           allocatedSandbox.TTL,
			CreateIndex:   index,
			CreateTime:    now,
			ModifyIndex:   index,
			ModifyTime:    now,
		}
		if err := txn.Insert(TableSandboxVolumes, vol); err != nil {
			return err
		}
		if err := txn.Insert(tableIndex,
			&IndexEntry{TableSandboxVolumes, index}); err != nil {
			return fmt.Errorf("index update failed: %w", err)
		}
	}
	return nil
}

func (s *StateStore) updateSandboxVolumeClaim(txn Txn, index uint64, now int64, old *structs.SandboxVolume, alloc *structs.Allocation) error {
	allocID := alloc.ID
	var delete bool

	if alloc.ClientTerminalStatus() {
		if !slices.Contains(old.AllocIDs, allocID) {
			return nil // already freed
		}
		delete = true
	} else if slices.Contains(old.AllocIDs, alloc.ID) {
		return nil // no update to make
	}

	vol := old.Copy()
	if delete {
		vol.AllocIDs = slices.DeleteFunc(vol.AllocIDs,
			func(id string) bool { return id == alloc.ID })
	} else {
		vol.AllocIDs = append(vol.AllocIDs, allocID)
	}
	vol.ModifyIndex = index
	vol.ModifyTime = now

	if err := txn.Insert(TableSandboxVolumes, vol); err != nil {
		return err
	}
	if err := txn.Insert(tableIndex,
		&IndexEntry{TableSandboxVolumes, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}
	return nil
}

// UpsertSandboxVolume should generally only be used for testing
func (s *StateStore) UpsertSandboxVolume(index uint64, sandbox *structs.SandboxVolume) error {
	txn := s.db.WriteTxnMsgT(structs.HostVolumeRegisterRequestType, index)
	defer txn.Abort()

	err := s.upsertSandboxVolumeTxn(txn, index, sandbox)
	if err == nil {
		txn.Commit()
	}
	return err
}

func (s *StateStore) upsertSandboxVolumeTxn(txn Txn, index uint64, sandbox *structs.SandboxVolume) error {
	if err := txn.Insert(TableSandboxVolumes, sandbox); err != nil {
		return err
	}
	if err := txn.Insert(tableIndex,
		&IndexEntry{TableSandboxVolumes, index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}
	return nil
}
