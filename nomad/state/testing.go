// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"math"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStateStore(t testing.TB) *StateStore {
	config := &StateStoreConfig{
		Logger:             testlog.HCLogger(t),
		Region:             "global",
		JobTrackedVersions: structs.JobDefaultTrackedVersions,
	}
	state, err := NewStateStore(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func TestStateStorePublisher(t testing.TB) *StateStoreConfig {
	return &StateStoreConfig{
		Logger:          testlog.HCLogger(t),
		Region:          "global",
		EnablePublisher: true,
	}
}
func TestStateStoreCfg(t testing.TB, cfg *StateStoreConfig) *StateStore {
	state, err := NewStateStore(cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

// CreateTestCSIPlugin is a helper that generates the node + fingerprint results necessary
// to create a CSIPlugin by directly inserting into the state store. The plugin requires a
// controller.
func CreateTestCSIPlugin(s *StateStore, id string) func() {
	return createTestCSIPlugin(s, id, true)
}

// CreateTestCSIPluginNodeOnly is a helper that generates the node + fingerprint results
// necessary to create a CSIPlugin by directly inserting into the state store. The plugin
// does not require a controller. In tests that exercise volume registration, this prevents
// an error attempting to RPC the node.
func CreateTestCSIPluginNodeOnly(s *StateStore, id string) func() {
	return createTestCSIPlugin(s, id, false)
}

func createTestCSIPlugin(s *StateStore, id string, requiresController bool) func() {
	// Create some nodes
	ns := make([]*structs.Node, 3)
	for i := range ns {
		n := mock.Node()
		n.Attributes["nomad.version"] = "0.11.0"
		ns[i] = n
	}

	// Install healthy plugin fingerprinting results
	ns[0].CSIControllerPlugins = map[string]*structs.CSIInfo{
		id: {
			PluginID:                 id,
			AllocID:                  uuid.Generate(),
			Healthy:                  true,
			HealthDescription:        "healthy",
			RequiresControllerPlugin: requiresController,
			RequiresTopologies:       false,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsReadOnlyAttach:           true,
				SupportsAttachDetach:             true,
				SupportsListVolumes:              true,
				SupportsListVolumesAttachedNodes: false,
				SupportsCreateDeleteSnapshot:     true,
				SupportsListSnapshots:            true,
			},
		},
	}

	// Install healthy plugin fingerprinting results
	for _, n := range ns[1:] {
		n.CSINodePlugins = map[string]*structs.CSIInfo{
			id: {
				PluginID:                 id,
				AllocID:                  uuid.Generate(),
				Healthy:                  true,
				HealthDescription:        "healthy",
				RequiresControllerPlugin: requiresController,
				RequiresTopologies:       false,
				NodeInfo: &structs.CSINodeInfo{
					ID:                      n.ID,
					MaxVolumes:              64,
					RequiresNodeStageVolume: true,
				},
			},
		}
	}

	// Insert them into the state store
	index := uint64(999)
	for _, n := range ns {
		index++
		s.UpsertNode(structs.MsgTypeTestSetup, index, n)
	}

	ids := make([]string, len(ns))
	for i, n := range ns {
		ids[i] = n.ID
	}

	// Return cleanup function that deletes the nodes
	return func() {
		index++
		s.DeleteNode(structs.MsgTypeTestSetup, index, ids)
	}
}

func TestBadCSIState(t testing.TB, store *StateStore) error {

	pluginID := "org.democratic-csi.nfs"

	controllerInfo := func(isHealthy bool) map[string]*structs.CSIInfo {
		desc := "healthy"
		if !isHealthy {
			desc = "failed fingerprinting with error"
		}
		return map[string]*structs.CSIInfo{
			pluginID: {
				PluginID:                 pluginID,
				AllocID:                  uuid.Generate(),
				Healthy:                  isHealthy,
				HealthDescription:        desc,
				RequiresControllerPlugin: true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsReadOnlyAttach: true,
					SupportsAttachDetach:   true,
				},
			},
		}
	}

	nodeInfo := func(nodeName string, isHealthy bool) map[string]*structs.CSIInfo {
		desc := "healthy"
		if !isHealthy {
			desc = "failed fingerprinting with error"
		}
		return map[string]*structs.CSIInfo{
			pluginID: {
				PluginID:                 pluginID,
				AllocID:                  uuid.Generate(),
				Healthy:                  isHealthy,
				HealthDescription:        desc,
				RequiresControllerPlugin: true,
				NodeInfo: &structs.CSINodeInfo{
					ID:                      nodeName,
					MaxVolumes:              math.MaxInt64,
					RequiresNodeStageVolume: true,
				},
			},
		}
	}

	nodes := make([]*structs.Node, 3)
	for i := range nodes {
		n := mock.Node()
		n.Attributes["nomad.version"] = "1.2.4"
		nodes[i] = n
	}

	nodes[0].CSIControllerPlugins = controllerInfo(true)
	nodes[0].CSINodePlugins = nodeInfo("nomad-client0", true)

	drainID := uuid.Generate()

	// drained node
	nodes[1].CSIControllerPlugins = controllerInfo(false)
	nodes[1].CSINodePlugins = nodeInfo("nomad-client1", false)

	nodes[1].LastDrain = &structs.DrainMetadata{
		StartedAt:  time.Now().Add(-10 * time.Minute),
		UpdatedAt:  time.Now().Add(-30 * time.Second),
		Status:     structs.DrainStatusComplete,
		AccessorID: drainID,
	}
	nodes[1].SchedulingEligibility = structs.NodeSchedulingIneligible

	// previously drained but now eligible
	nodes[2].CSIControllerPlugins = controllerInfo(true)
	nodes[2].CSINodePlugins = nodeInfo("nomad-client2", true)
	nodes[2].LastDrain = &structs.DrainMetadata{
		StartedAt:  time.Now().Add(-15 * time.Minute),
		UpdatedAt:  time.Now().Add(-5 * time.Minute),
		Status:     structs.DrainStatusComplete,
		AccessorID: drainID,
	}
	nodes[2].SchedulingEligibility = structs.NodeSchedulingEligible

	// Insert nodes into the state store
	index := uint64(999)
	for _, n := range nodes {
		index++
		err := store.UpsertNode(structs.MsgTypeTestSetup, index, n)
		if err != nil {
			return err
		}
	}

	allocID0 := uuid.Generate() // nil alloc
	allocID2 := uuid.Generate() // nil alloc

	alloc1 := mock.Alloc()
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	alloc1.DesiredStatus = structs.AllocDesiredStatusRun

	// Insert allocs into the state store
	err := store.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc1})
	if err != nil {
		return err
	}

	vol := &structs.CSIVolume{
		ID:             "csi-volume-nfs0",
		Name:           "csi-volume-nfs0",
		ExternalID:     "csi-volume-nfs0",
		Namespace:      "default",
		AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		MountOptions: &structs.CSIMountOptions{
			MountFlags: []string{"noatime"},
		},
		Context: map[string]string{
			"node_attach_driver": "nfs",
			"provisioner_driver": "nfs-client",
			"server":             "192.168.56.69",
		},
		Capacity:             0,
		RequestedCapacityMin: 107374182,
		RequestedCapacityMax: 107374182,
		RequestedCapabilities: []*structs.CSIVolumeCapability{
			{
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
			{
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
			},
			{
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				AccessMode:     structs.CSIVolumeAccessModeSingleNodeReader,
			},
		},
		WriteAllocs: map[string]*structs.Allocation{
			allocID0:  nil,
			alloc1.ID: nil,
			allocID2:  nil,
		},
		WriteClaims: map[string]*structs.CSIVolumeClaim{
			allocID0: {
				AllocationID:   allocID0,
				NodeID:         nodes[0].ID,
				Mode:           structs.CSIVolumeClaimWrite,
				AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				State:          structs.CSIVolumeClaimStateTaken,
			},
			alloc1.ID: {
				AllocationID:   alloc1.ID,
				NodeID:         nodes[1].ID,
				Mode:           structs.CSIVolumeClaimWrite,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				State:          structs.CSIVolumeClaimStateTaken,
			},
			allocID2: {
				AllocationID:   allocID2,
				NodeID:         nodes[2].ID,
				Mode:           structs.CSIVolumeClaimWrite,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
				State:          structs.CSIVolumeClaimStateTaken,
			},
		},
		Schedulable:         true,
		PluginID:            pluginID,
		Provider:            pluginID,
		ProviderVersion:     "1.4.3",
		ControllerRequired:  true,
		ControllersHealthy:  2,
		ControllersExpected: 2,
		NodesHealthy:        2,
		NodesExpected:       0,
	}
	vol = vol.Copy() // canonicalize

	err = store.UpsertCSIVolume(index, []*structs.CSIVolume{vol})
	if err != nil {
		return err
	}

	return nil
}
