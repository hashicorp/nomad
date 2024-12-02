// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeRegisterCommand) hostVolumeRegister(client *api.Client, ast *ast.File) int {
	vol, err := decodeHostVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	req := &api.HostVolumeRegisterRequest{
		Volumes: []*api.HostVolume{vol},
	}
	vols, _, err := client.HostVolumes().Register(req, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering volume: %s", err))
		return 1
	}
	for _, vol := range vols {
		// note: the command only ever returns 1 volume from the API
		c.Ui.Output(fmt.Sprintf(
			"Registered host volume %s with ID %s", vol.Name, vol.ID))
	}

	return 0
}
