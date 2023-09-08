// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
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

	c.Ui.Output(fmt.Sprintf("Volume %q registered", vol.ID))
	return 0
}

func csiDecodeVolume(input *ast.File) (*api.CSIVolume, error) {
	var err error
	vol := &api.CSIVolume{}

	list, ok := input.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	// Decode the full thing into a map[string]interface for ease
	var m map[string]interface{}
	err = hcl.DecodeObject(&m, list)
	if err != nil {
		return nil, err
	}

	// Need to manually parse these fields
	delete(m, "capability")
	delete(m, "mount_options")
	delete(m, "capacity_max")
	delete(m, "capacity_min")
	delete(m, "topology_request")
	delete(m, "type")

	// Decode the rest
	err = mapstructure.WeakDecode(m, vol)
	if err != nil {
		return nil, err
	}

	capacityMin, err := parseCapacityBytes(list.Filter("capacity_min"))
	if err != nil {
		return nil, fmt.Errorf("invalid capacity_min: %v", err)
	}
	vol.RequestedCapacityMin = capacityMin
	capacityMax, err := parseCapacityBytes(list.Filter("capacity_max"))
	if err != nil {
		return nil, fmt.Errorf("invalid capacity_max: %v", err)
	}
	vol.RequestedCapacityMax = capacityMax

	capObj := list.Filter("capability")
	if len(capObj.Items) > 0 {

		for _, o := range capObj.Elem().Items {
			valid := []string{"access_mode", "attachment_mode"}
			if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
				return nil, err
			}

			ot, ok := o.Val.(*ast.ObjectType)
			if !ok {
				break
			}

			var m map[string]interface{}
			if err := hcl.DecodeObject(&m, ot.List); err != nil {
				return nil, err
			}
			var cap *api.CSIVolumeCapability
			if err := mapstructure.WeakDecode(&m, &cap); err != nil {
				return nil, err
			}

			vol.RequestedCapabilities = append(vol.RequestedCapabilities, cap)
		}
	}

	mObj := list.Filter("mount_options")
	if len(mObj.Items) > 0 {

		for _, o := range mObj.Elem().Items {
			valid := []string{"fs_type", "mount_flags"}
			if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
				return nil, err
			}

			ot, ok := o.Val.(*ast.ObjectType)
			if !ok {
				break
			}
			var opts *api.CSIMountOptions
			if err := hcl.DecodeObject(&opts, ot.List); err != nil {
				return nil, err
			}
			vol.MountOptions = opts
			break
		}
	}

	requestedTopos := list.Filter("topology_request")
	if len(requestedTopos.Items) > 0 {

		vol.RequestedTopologies = &api.CSITopologyRequest{}

		for _, o := range requestedTopos.Elem().Items {
			if err := helper.CheckHCLKeys(o.Val, []string{"preferred", "required"}); err != nil {
				return nil, err
			}
			ot, ok := o.Val.(*ast.ObjectType)
			if !ok {
				break
			}

			// topology_request -> required|preferred -> []topology -> []segments (kv)
			decoded := map[string][]map[string][]map[string][]map[string]string{}
			if err := hcl.DecodeObject(&decoded, ot.List); err != nil {
				return nil, err
			}

			getTopologies := func(topKey string) []*api.CSITopology {
				for _, topo := range decoded[topKey] {
					var topos []*api.CSITopology
					for _, segments := range topo["topology"] {
						for _, segment := range segments["segments"] {
							if len(segment) > 0 {
								topos = append(topos, &api.CSITopology{Segments: segment})
							}
						}
					}
					if len(topos) > 0 {
						return topos
					}
				}
				return nil
			}

			vol.RequestedTopologies.Required = getTopologies("required")
			vol.RequestedTopologies.Preferred = getTopologies("preferred")
		}
	}

	return vol, nil
}

func parseCapacityBytes(cap *ast.ObjectList) (int64, error) {
	if len(cap.Items) > 0 {
		for _, o := range cap.Elem().Items {
			lit, ok := o.Val.(*ast.LiteralType)
			if !ok {
				break
			}
			literal := strings.Trim(lit.Token.Text, "\"")
			if literal == "" {
				return 0, nil
			}
			b, err := humanize.ParseBytes(literal)
			if err != nil {
				return 0, fmt.Errorf("could not parse value as bytes: %v", err)
			}
			return int64(b), err
		}
	}
	return 0, nil
}
