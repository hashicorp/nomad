// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestHostVolume_Copy(t *testing.T) {
	ci.Parallel(t)

	out := (*HostVolume)(nil).Copy()
	must.Nil(t, out)

	vol := &HostVolume{
		Namespace: DefaultNamespace,
		ID:        uuid.Generate(),
		Name:      "example",
		PluginID:  "example-plugin",
		NodePool:  NodePoolDefault,
		NodeID:    uuid.Generate(),
		Constraints: []*Constraint{{
			LTarget: "${meta.rack}",
			RTarget: "r1",
			Operand: "=",
		}},
		CapacityBytes: 150000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeWriter,
		}},
		Parameters: map[string]string{"foo": "bar"},
	}

	out = vol.Copy()
	must.Eq(t, vol, out)

	out.Allocations = []*AllocListStub{{ID: uuid.Generate()}}
	out.Constraints[0].LTarget = "${meta.node_class}"
	out.RequestedCapabilities = append(out.RequestedCapabilities, &HostVolumeCapability{
		AttachmentMode: HostVolumeAttachmentModeBlockDevice,
		AccessMode:     HostVolumeAccessModeSingleNodeMultiWriter,
	})
	out.Parameters["foo"] = "baz"

	must.Nil(t, vol.Allocations)
	must.Eq(t, "${meta.rack}", vol.Constraints[0].LTarget)
	must.Len(t, 1, vol.RequestedCapabilities)
	must.Eq(t, "bar", vol.Parameters["foo"])
}

func TestHostVolume_Validate(t *testing.T) {
	ci.Parallel(t)

	invalid := &HostVolume{RequestedCapabilities: []*HostVolumeCapability{
		{AttachmentMode: "foo"}}}
	err := invalid.Validate()
	must.EqError(t, err, `2 errors occurred:
	* missing name
	* invalid attachment mode: "foo"

`)

	invalid = &HostVolume{}
	err = invalid.Validate()
	// single error should be flattened
	must.EqError(t, err, "missing name")

	invalid = &HostVolume{
		ID:       "../../not-a-uuid",
		Name:     "example",
		PluginID: "example-plugin",
		Constraints: []*Constraint{{
			RTarget: "r1",
			Operand: "=",
		}},
		RequestedCapacityMinBytes: 200000,
		RequestedCapacityMaxBytes: 100000,
		RequestedCapabilities: []*HostVolumeCapability{
			{
				AttachmentMode: HostVolumeAttachmentModeFilesystem,
				AccessMode:     HostVolumeAccessModeSingleNodeWriter,
			},
			{
				AttachmentMode: "bad",
				AccessMode:     "invalid",
			},
		},
	}
	err = invalid.Validate()
	must.EqError(t, err, `4 errors occurred:
	* invalid ID "../../not-a-uuid"
	* capacity_max (100000) must be larger than capacity_min (200000)
	* invalid attachment mode: "bad"
	* invalid constraint: 1 error occurred:
	* No LTarget provided but is required by constraint



`)

	vol := &HostVolume{
		Namespace: DefaultNamespace,
		ID:        uuid.Generate(),
		Name:      "example",
		PluginID:  "example-plugin",
		NodePool:  NodePoolDefault,
		NodeID:    uuid.Generate(),
		Constraints: []*Constraint{{
			LTarget: "${meta.rack}",
			RTarget: "r1",
			Operand: "=",
		}},
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 200000,
		CapacityBytes:             150000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeWriter,
		}},
		Parameters: map[string]string{"foo": "bar"},
	}
	must.NoError(t, vol.Validate())
}

func TestHostVolume_ValidateUpdate(t *testing.T) {
	ci.Parallel(t)

	vol := &HostVolume{
		NodePool:                  NodePoolDefault,
		NodeID:                    uuid.Generate(),
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 120000,
		Parameters:                map[string]string{"baz": "qux"},
	}
	err := vol.ValidateUpdate(nil)
	must.NoError(t, err)

	existing := &HostVolume{
		NodePool:                  "prod",
		NodeID:                    uuid.Generate(),
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 200000,
		CapacityBytes:             150000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeWriter,
		}},
		Parameters: map[string]string{"foo": "bar"},
		Allocations: []*AllocListStub{
			{ID: "6bd66bfa"},
			{ID: "7032e570"},
		},
	}

	err = vol.ValidateUpdate(existing)
	must.EqError(t, err, `4 errors occurred:
	* cannot update a volume in use: claimed by allocs (6bd66bfa, 7032e570)
	* node ID cannot be updated
	* node pool cannot be updated
	* capacity_max (120000) cannot be less than existing provisioned capacity (150000)

`)

}

func TestHostVolume_CanonicalizeForCreate(t *testing.T) {
	now := time.Now()
	vol := &HostVolume{
		CapacityBytes: 100000,
		HostPath:      "/etc/passwd",
		Allocations: []*AllocListStub{
			{ID: "6bd66bfa"},
			{ID: "7032e570"},
		},
	}
	vol.CanonicalizeForCreate(nil, now)

	must.NotEq(t, "", vol.ID)
	must.Eq(t, now.UnixNano(), vol.CreateTime)
	must.Eq(t, now.UnixNano(), vol.ModifyTime)
	must.Eq(t, HostVolumeStatePending, vol.State)
	must.Nil(t, vol.Allocations)
	must.Eq(t, "", vol.HostPath)
	must.Zero(t, vol.CapacityBytes)

	vol = &HostVolume{
		ID:                        "82f357d6-a5ec-11ef-9e36-3f9884222736",
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 500000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeMultiWriter,
		}},
	}
	existing := &HostVolume{
		ID:                        "82f357d6-a5ec-11ef-9e36-3f9884222736",
		PluginID:                  "example_plugin",
		NodePool:                  "prod",
		NodeID:                    uuid.Generate(),
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 200000,
		CapacityBytes:             150000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeWriter,
		}},
		Constraints: []*Constraint{{
			LTarget: "${meta.rack}",
			RTarget: "r1",
			Operand: "=",
		}},
		Parameters: map[string]string{"foo": "bar"},
		Allocations: []*AllocListStub{
			{ID: "6bd66bfa"},
			{ID: "7032e570"},
		},
		HostPath:   "/var/nomad/alloc_mounts/82f357d6.ext4",
		CreateTime: 1,
	}

	vol.CanonicalizeForCreate(existing, now)
	must.Eq(t, existing.ID, vol.ID)
	must.Eq(t, existing.PluginID, vol.PluginID)
	must.Eq(t, existing.NodePool, vol.NodePool)
	must.Eq(t, existing.NodeID, vol.NodeID)
	must.Eq(t, []*Constraint{{
		LTarget: "${meta.rack}",
		RTarget: "r1",
		Operand: "=",
	}}, vol.Constraints)
	must.Eq(t, 100000, vol.RequestedCapacityMinBytes)
	must.Eq(t, 500000, vol.RequestedCapacityMaxBytes)
	must.Eq(t, 150000, vol.CapacityBytes)

	must.Eq(t, []*HostVolumeCapability{{
		AttachmentMode: HostVolumeAttachmentModeFilesystem,
		AccessMode:     HostVolumeAccessModeSingleNodeMultiWriter,
	}}, vol.RequestedCapabilities)

	must.Eq(t, "/var/nomad/alloc_mounts/82f357d6.ext4", vol.HostPath)
	must.Eq(t, HostVolumeStatePending, vol.State)

	must.Eq(t, existing.CreateTime, vol.CreateTime)
	must.Eq(t, now.UnixNano(), vol.ModifyTime)
	must.Nil(t, vol.Allocations)
}

func TestHostVolume_CanonicalizeForRegister(t *testing.T) {
	now := time.Now()
	nodeID := uuid.Generate()
	vol := &HostVolume{
		NodeID:        nodeID,
		CapacityBytes: 100000,
		HostPath:      "/etc/passwd",
		Allocations: []*AllocListStub{
			{ID: "6bd66bfa"},
			{ID: "7032e570"},
		},
	}
	vol.CanonicalizeForRegister(nil, now)

	must.NotEq(t, "", vol.ID)
	must.Eq(t, now.UnixNano(), vol.CreateTime)
	must.Eq(t, now.UnixNano(), vol.ModifyTime)
	must.Eq(t, HostVolumeStatePending, vol.State)
	must.Nil(t, vol.Allocations)
	must.Eq(t, "/etc/passwd", vol.HostPath)
	must.Eq(t, nodeID, vol.NodeID)
	must.Eq(t, 100000, vol.CapacityBytes)

	vol = &HostVolume{
		ID:                        "82f357d6-a5ec-11ef-9e36-3f9884222736",
		PluginID:                  "example_plugin.v2",
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 500000,
		CapacityBytes:             200000,
		NodePool:                  "infra",
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeMultiWriter,
		}},
		HostPath: "/var/nomad/alloc_mounts/82f357d6.ext4",
	}
	existing := &HostVolume{
		ID:                        "82f357d6-a5ec-11ef-9e36-3f9884222736",
		PluginID:                  "example_plugin.v1",
		NodePool:                  "prod",
		NodeID:                    uuid.Generate(),
		RequestedCapacityMinBytes: 100000,
		RequestedCapacityMaxBytes: 200000,
		CapacityBytes:             150000,
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem,
			AccessMode:     HostVolumeAccessModeSingleNodeWriter,
		}},
		Constraints: []*Constraint{{
			LTarget: "${meta.rack}",
			RTarget: "r1",
			Operand: "=",
		}},
		Parameters: map[string]string{"foo": "bar"},
		Allocations: []*AllocListStub{
			{ID: "6bd66bfa"},
			{ID: "7032e570"},
		},
		HostPath:   "/var/nomad/alloc_mounts/82f357d6.img",
		CreateTime: 1,
	}

	vol.CanonicalizeForRegister(existing, now)

	must.Eq(t, existing.ID, vol.ID)
	must.Eq(t, "example_plugin.v2", vol.PluginID)
	must.Eq(t, "infra", vol.NodePool)
	must.Eq(t, existing.NodeID, vol.NodeID)
	must.Eq(t, []*Constraint{{
		LTarget: "${meta.rack}",
		RTarget: "r1",
		Operand: "=",
	}}, vol.Constraints)
	must.Eq(t, 100000, vol.RequestedCapacityMinBytes)
	must.Eq(t, 500000, vol.RequestedCapacityMaxBytes)
	must.Eq(t, 200000, vol.CapacityBytes)

	must.Eq(t, []*HostVolumeCapability{{
		AttachmentMode: HostVolumeAttachmentModeFilesystem,
		AccessMode:     HostVolumeAccessModeSingleNodeMultiWriter,
	}}, vol.RequestedCapabilities)

	must.Eq(t, "/var/nomad/alloc_mounts/82f357d6.ext4", vol.HostPath)
	must.Eq(t, HostVolumeStatePending, vol.State)

	must.Eq(t, existing.CreateTime, vol.CreateTime)
	must.Eq(t, now.UnixNano(), vol.ModifyTime)
	must.Nil(t, vol.Allocations)

}
