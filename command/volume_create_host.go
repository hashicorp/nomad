// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/mapstructure"
)

func (c *VolumeCreateCommand) hostVolumeCreate(client *api.Client, ast *ast.File) int {
	vol, err := decodeHostVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
		return 1
	}

	req := &api.HostVolumeCreateRequest{
		Volumes: []*api.HostVolume{vol},
	}
	vols, _, err := client.HostVolumes().Create(req, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating volume: %s", err))
		return 1
	}
	for _, vol := range vols {
		// note: the command only ever returns 1 volume from the API
		c.Ui.Output(fmt.Sprintf(
			"Created host volume %s with ID %s", vol.Name, vol.ID))
	}

	// TODO(1.10.0): monitor so we can report when the node has fingerprinted

	return 0
}

func decodeHostVolume(input *ast.File) (*api.HostVolume, error) {
	var err error
	vol := &api.HostVolume{}

	list, ok := input.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("error parsing: root should be an object")
	}

	// Decode the full thing into a map[string]interface for ease
	var m map[string]any
	err = hcl.DecodeObject(&m, list)
	if err != nil {
		return nil, err
	}

	// Need to manually parse these fields/blocks
	delete(m, "capability")
	delete(m, "constraint")
	delete(m, "capacity_max")
	delete(m, "capacity_min")
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
	vol.RequestedCapacityMinBytes = capacityMin
	capacityMax, err := parseCapacityBytes(list.Filter("capacity_max"))
	if err != nil {
		return nil, fmt.Errorf("invalid capacity_max: %v", err)
	}
	vol.RequestedCapacityMaxBytes = capacityMax

	if o := list.Filter("constraint"); len(o.Items) > 0 {
		if err := parseConstraints(&vol.Constraints, o); err != nil {
			return nil, fmt.Errorf("invalid constraint: %v", err)
		}
	}
	if o := list.Filter("capability"); len(o.Items) > 0 {
		if err := parseHostVolumeCapabilities(&vol.RequestedCapabilities, o); err != nil {
			return nil, fmt.Errorf("invalid capability: %v", err)
		}
	}

	return vol, nil
}

func parseHostVolumeCapabilities(result *[]*api.HostVolumeCapability, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		valid := []string{"access_mode", "attachment_mode"}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		ot, ok := o.Val.(*ast.ObjectType)
		if !ok {
			break
		}

		var m map[string]any
		if err := hcl.DecodeObject(&m, ot.List); err != nil {
			return err
		}
		var cap *api.HostVolumeCapability
		if err := mapstructure.WeakDecode(&m, &cap); err != nil {
			return err
		}

		*result = append(*result, cap)
	}

	return nil
}

func parseConstraints(result *[]*api.Constraint, list *ast.ObjectList) error {
	for _, o := range list.Elem().Items {
		valid := []string{
			"attribute",
			"distinct_hosts",
			"distinct_property",
			"operator",
			"regexp",
			"set_contains",
			"value",
			"version",
			"semver",
		}
		if err := helper.CheckHCLKeys(o.Val, valid); err != nil {
			return err
		}

		var m map[string]any
		if err := hcl.DecodeObject(&m, o.Val); err != nil {
			return err
		}

		m["LTarget"] = m["attribute"]
		m["RTarget"] = m["value"]
		m["Operand"] = m["operator"]

		// If "version" is provided, set the operand
		// to "version" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintVersion]; ok {
			m["Operand"] = api.ConstraintVersion
			m["RTarget"] = constraint
		}

		// If "semver" is provided, set the operand
		// to "semver" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintSemver]; ok {
			m["Operand"] = api.ConstraintSemver
			m["RTarget"] = constraint
		}

		// If "regexp" is provided, set the operand
		// to "regexp" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintRegex]; ok {
			m["Operand"] = api.ConstraintRegex
			m["RTarget"] = constraint
		}

		// If "set_contains" is provided, set the operand
		// to "set_contains" and the value to the "RTarget"
		if constraint, ok := m[api.ConstraintSetContains]; ok {
			m["Operand"] = api.ConstraintSetContains
			m["RTarget"] = constraint
		}

		if value, ok := m[api.ConstraintDistinctHosts]; ok {
			enabled, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("distinct_hosts should be set to true or false; %v", err)
			}

			// If it is not enabled, skip the constraint.
			if !enabled {
				continue
			}

			m["Operand"] = api.ConstraintDistinctHosts
			m["RTarget"] = strconv.FormatBool(enabled)
		}

		if property, ok := m[api.ConstraintDistinctProperty]; ok {
			m["Operand"] = api.ConstraintDistinctProperty
			m["LTarget"] = property
		}

		// Build the constraint
		var c api.Constraint
		if err := mapstructure.WeakDecode(m, &c); err != nil {
			return err
		}
		if c.Operand == "" {
			c.Operand = "="
		}

		*result = append(*result, &c)
	}

	return nil
}

// parseBool takes an interface value and tries to convert it to a boolean and
// returns an error if the type can't be converted.
func parseBool(value any) (bool, error) {
	var enabled bool
	var err error
	switch data := value.(type) {
	case string:
		enabled, err = strconv.ParseBool(data)
	case bool:
		enabled = data
	default:
		err = fmt.Errorf("%v couldn't be converted to boolean value", value)
	}

	return enabled, err
}
