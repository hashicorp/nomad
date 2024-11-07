// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func HostVolume() *structs.HostVolume {

	volID := uuid.Generate()
	vol := &structs.HostVolume{
		Namespace: structs.DefaultNamespace,
		ID:        volID,
		Name:      "example",
		PluginID:  "example-plugin",
		NodePool:  structs.NodePoolDefault,
		NodeID:    uuid.Generate(),
		Constraints: []*structs.Constraint{
			{
				LTarget: "${meta.rack}",
				RTarget: "r1",
				Operand: "=",
			},
		},
		RequestedCapacityMin: 100000,
		RequestedCapacityMax: 200000,
		Capacity:             150000,
		RequestedCapabilities: []*structs.HostVolumeCapability{
			{
				AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
				AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
			},
		},
		Parameters: map[string]string{"foo": "bar"},
		HostPath:   "/var/data/nomad/alloc_mounts/" + volID,
		State:      structs.HostVolumeStatePending,
	}
	return vol
}
