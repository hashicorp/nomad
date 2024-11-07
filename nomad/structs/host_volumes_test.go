// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

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
		Capacity: 150000,
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
		AccessMode:     HostVolumeAccessModeMultiNodeReader,
	})
	out.Parameters["foo"] = "baz"

	must.Nil(t, vol.Allocations)
	must.Eq(t, "${meta.rack}", vol.Constraints[0].LTarget)
	must.Len(t, 1, vol.RequestedCapabilities)
	must.Eq(t, "bar", vol.Parameters["foo"])
}
