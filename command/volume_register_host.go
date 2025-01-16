// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeRegisterCommand) hostVolumeRegister(client *api.Client, ast *ast.File, override bool) int {
	vol, err := decodeHostVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}
	if vol.NodeID == "" {
		c.Ui.Error("Node ID is required for registering")
		return 1
	}

	req := &api.HostVolumeRegisterRequest{
		Volume:         vol,
		PolicyOverride: override,
	}
	resp, _, err := client.HostVolumes().Register(req, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering volume: %s", err))
		return 1
	}
	vol = resp.Volume

	if resp.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(
				fmt.Sprintf("[bold][yellow]Volume Warnings:\n%s[reset]\n", resp.Warnings)))
	}

	c.Ui.Output(fmt.Sprintf(
		"Registered host volume %s with ID %s", vol.Name, vol.ID))

	return 0
}
