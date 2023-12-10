// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestClientCSINodeExpandVolumeRequest_Validate(t *testing.T) {
	req := &ClientCSINodeExpandVolumeRequest{
		PluginID:   "plug-id",
		VolumeID:   "vol-id",
		ExternalID: "ext-id",
		Claim: &structs.CSIVolumeClaim{
			AllocationID: "alloc-id",
		},
	}
	err := req.Validate()
	must.NoError(t, err)

	req.PluginID = ""
	err = req.Validate()
	must.ErrorContains(t, err, "PluginID is required")

	req.VolumeID = ""
	err = req.Validate()
	must.ErrorContains(t, err, "VolumeID is required")

	req.ExternalID = ""
	err = req.Validate()
	must.ErrorContains(t, err, "ExternalID is required")

	req.Claim.AllocationID = ""
	err = req.Validate()
	must.ErrorContains(t, err, "Claim.AllocationID is required")

	req.Claim = nil
	err = req.Validate()
	must.ErrorContains(t, err, "Claim is required")
}
