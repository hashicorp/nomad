// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func HostVolumeRequest() *structs.HostVolume {
	vol := &structs.HostVolume{
		Namespace: structs.DefaultNamespace,
		Name:      "example",
		PluginID:  "example-plugin",
		NodePool:  structs.NodePoolDefault,
		Constraints: []*structs.Constraint{
			{
				LTarget: "${meta.rack}",
				RTarget: "r1",
				Operand: "=",
			},
		},
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 200000,
		RequestedCapabilities: []*structs.HostVolumeCapability{
			{
				AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
				AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
			},
		},
		Parameters: map[string]string{"foo": "bar"},
		State:      structs.HostVolumeStatePending,
	}
	return vol

}

func HostVolume() *structs.HostVolume {
	volID := uuid.Generate()
	vol := HostVolumeRequest()
	vol.ID = volID
	vol.NodeID = uuid.Generate()
	vol.CapacityBytes = 150000
	vol.HostPath = "/var/data/nomad/alloc_mounts/" + volID
	return vol
}
