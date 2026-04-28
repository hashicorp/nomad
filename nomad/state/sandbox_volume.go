package state

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

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
	return nil
}

func (s *StateStore) claimSandboxVolume(txn Txn, index uint64, now int64, sandboxName string, alloc *structs.Allocation) error {
	obj, err := txn.First(TableSandboxVolumes, indexClaimID,
		alloc.Namespace, sandboxName, alloc.ID)
	if err != nil {
		return err
	}
	if obj != nil {
		old := obj.(*structs.SandboxVolume)
		return s.updateSandboxVolumeClaim(txn, index, now, old, alloc)
	}

	obj, err = txn.First(TableSandboxVolumes, indexNodeID,
		alloc.Namespace, sandboxName, alloc.NodeID)
	if err != nil {
		return err
	}
	var old *structs.SandboxVolume
	if obj != nil {
		old = obj.(*structs.SandboxVolume)
		return s.updateSandboxVolumeClaim(txn, index, now, old, alloc)
	}

	// create a new sandbox and claim it for the allocation
	vol := &structs.SandboxVolume{
		ID:            uuid.Generate(), // <-- TODO: no, we can't do this!!!!
		Namespace:     alloc.Namespace,
		Name:          sandboxName,
		AllocID:       alloc.ID,
		NodeID:        alloc.NodeID,
		CapacityBytes: 0, // ?????
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
	return nil
}

func (s *StateStore) updateSandboxVolumeClaim(txn Txn, index uint64, now int64, old *structs.SandboxVolume, alloc *structs.Allocation) error {

	allocID := alloc.ID
	if alloc.ClientTerminalStatus() {
		if old.AllocID == "" {
			return nil // already freed
		}
		allocID = ""
	} else if old.AllocID == alloc.ID {
		return nil // no update to make
	}

	vol := old.Copy()
	vol.AllocID = allocID
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
