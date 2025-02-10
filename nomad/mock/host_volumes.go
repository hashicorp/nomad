// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func HostVolumeRequest(ns string) *structs.HostVolume {
	vol := &structs.HostVolume{
		Namespace: ns,
		Name:      "example",
		PluginID:  "mkdir",
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
		Parameters:                map[string]string{"foo": "bar"},
		State:                     structs.HostVolumeStatePending,
	}
	return vol

}

func HostVolumeRequestForNode(ns string, node *structs.Node) *structs.HostVolume {
	vol := HostVolumeRequest(ns)
	vol.NodeID = node.ID
	vol.NodePool = node.NodePool
	return vol
}

func HostVolume() *structs.HostVolume {
	volID := uuid.Generate()
	vol := HostVolumeRequest(structs.DefaultNamespace)
	vol.ID = volID
	vol.NodeID = uuid.Generate()
	vol.CapacityBytes = 150000
	vol.HostPath = "/var/data/nomad/alloc_mounts/" + volID
	return vol
}

// TaskGroupHostVolumeClaim creates a claim for a given job, alloc and host
// volume request
func TaskGroupHostVolumeClaim(job *structs.Job, alloc *structs.Allocation, dhv *structs.HostVolume) *structs.TaskGroupHostVolumeClaim {
	return &structs.TaskGroupHostVolumeClaim{
		ID:            uuid.Generate(),
		Namespace:     structs.DefaultNamespace,
		JobID:         job.ID,
		TaskGroupName: job.TaskGroups[0].Name,
		AllocID:       alloc.ID,
		VolumeID:      dhv.ID,
		VolumeName:    dhv.Name,
		CreateIndex:   1000,
		ModifyIndex:   1000,
	}
}
