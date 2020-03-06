package command

import (
	"fmt"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
)

func (c *VolumeRegisterCommand) csiRegister(client *api.Client, ast *ast.File) int {
	vol, err := csiDecodeVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}
	_, err = client.CSIVolumes().Register(vol, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering volume: %s", err))
		return 1
	}

	return 0
}

// parseVolume is used to parse the quota specification from HCL
func csiDecodeVolume(input *ast.File) (*api.CSIVolume, error) {
	output := &api.CSIVolume{}
	err := hcl.DecodeObject(output, input)
	if err != nil {
		return nil, err
	}

	// api.CSIVolume doesn't have the type field, it's used only for dispatch in
	// parseVolumeType
	helper.RemoveEqualFold(&output.ExtraKeysHCL, "type")
	err = helper.UnusedKeys(output)
	if err != nil {
		return nil, err
	}

	return output, nil
}
