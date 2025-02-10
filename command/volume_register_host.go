// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeRegisterCommand) hostVolumeRegister(client *api.Client, ast *ast.File, override bool, volID string) int {
	vol, err := decodeHostVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}
	if vol.NodeID == "" {
		c.Ui.Error("Node ID is required for registering")
		return 1
	}
	if volID != "" {
		ns := c.namespace
		if vol.Namespace != "" {
			ns = vol.Namespace
		}
		stub, possible, err := getHostVolumeByPrefix(client, volID, ns)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not update existing volume: %s", err))
			return 1
		}
		if len(possible) > 0 {
			out, err := formatHostVolumes(possible, formatOpts{short: true})
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
				return 1
			}
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple volumes\n\n%s", out))
			return 1
		}
		vol.ID = stub.ID
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
