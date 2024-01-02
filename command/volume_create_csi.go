// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeCreateCommand) csiCreate(client *api.Client, ast *ast.File) int {
	vol, err := csiDecodeVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	vols, _, err := client.CSIVolumes().Create(vol, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating volume: %s", err))
		return 1
	}
	for _, vol := range vols {
		// note: the command only ever returns 1 volume from the API
		c.Ui.Output(fmt.Sprintf(
			"Created external volume %s with ID %s", vol.ExternalID, vol.ID))
	}

	return 0
}
