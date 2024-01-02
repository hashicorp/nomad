// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumewatcher

import (
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Create a client node with plugin info
func testNode(plugin *structs.CSIPlugin, s *state.StateStore) *structs.Node {
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early version
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		plugin.ID: {
			PluginID:                 plugin.ID,
			Healthy:                  true,
			RequiresControllerPlugin: plugin.ControllerRequired,
			NodeInfo:                 &structs.CSINodeInfo{},
		},
	}
	if plugin.ControllerRequired {
		node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			plugin.ID: {
				PluginID:                 plugin.ID,
				Healthy:                  true,
				RequiresControllerPlugin: true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsReadOnlyAttach:           true,
					SupportsAttachDetach:             true,
					SupportsListVolumes:              true,
					SupportsListVolumesAttachedNodes: false,
				},
			},
		}
	} else {
		node.CSIControllerPlugins = map[string]*structs.CSIInfo{}
	}
	s.UpsertNode(structs.MsgTypeTestSetup, 99, node)
	return node
}

// Create a test volume with existing claim info
func testVolume(plugin *structs.CSIPlugin, alloc *structs.Allocation, nodeID string) *structs.CSIVolume {
	vol := mock.CSIVolume(plugin)
	vol.ControllerRequired = plugin.ControllerRequired

	// these modes were set by the previous claim
	vol.AccessMode = structs.CSIVolumeAccessModeMultiNodeReader
	vol.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem

	vol.ReadAllocs = map[string]*structs.Allocation{alloc.ID: alloc}
	vol.ReadClaims = map[string]*structs.CSIVolumeClaim{
		alloc.ID: {
			AllocationID:   alloc.ID,
			NodeID:         nodeID,
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeReader,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			Mode:           structs.CSIVolumeClaimRead,
			State:          structs.CSIVolumeClaimStateTaken,
		},
	}
	return vol
}

type MockRPCServer struct {
	state *state.StateStore

	nextCSIUnpublishResponse *structs.CSIVolumeUnpublishResponse
	nextCSIUnpublishError    error
	countCSIUnpublish        int
}

func (srv *MockRPCServer) Unpublish(args *structs.CSIVolumeUnpublishRequest, reply *structs.CSIVolumeUnpublishResponse) error {
	reply = srv.nextCSIUnpublishResponse
	srv.countCSIUnpublish++
	return srv.nextCSIUnpublishError
}

func (srv *MockRPCServer) State() *state.StateStore { return srv.state }

type MockBatchingRPCServer struct {
	MockRPCServer
}

type MockStatefulRPCServer struct {
	MockRPCServer
}
