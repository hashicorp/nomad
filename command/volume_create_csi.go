// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeCreateCommand) csiCreate(client *api.Client, ast *ast.File, override bool) int {
	vol, err := csiDecodeVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	resp, _, err := client.CSIVolumes().Create(vol, override, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating volume: %s", err))
		return 1
	}

	if resp.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(
				fmt.Sprintf("[bold][yellow]Volume Warnings:\n%s[reset]\n", resp.Warnings)))
	}
	for _, vol := range resp.Volumes {
		// note: the command only ever returns 1 volume from the API
		c.Ui.Output(fmt.Sprintf(
			"Created external volume %s with ID %s", vol.ExternalID, vol.ID))
	}

	return 0
}
