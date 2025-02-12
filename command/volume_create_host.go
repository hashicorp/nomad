// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/go-glint"
	"github.com/mitchellh/go-glint/components"
	"github.com/mitchellh/mapstructure"
)

func (c *VolumeCreateCommand) hostVolumeCreate(
	client *api.Client, ast *ast.File, detach, verbose, override bool, volID string) int {

	vol, err := decodeHostVolume(ast)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error decoding the volume definition: %s", err))
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

	req := &api.HostVolumeCreateRequest{
		Volume:         vol,
		PolicyOverride: override,
	}
	resp, _, err := client.HostVolumes().Create(req, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating volume: %s", err))
		return 1
	}
	vol = resp.Volume

	if resp.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(
				fmt.Sprintf("[bold][yellow]Volume Warnings:\n%s[reset]\n", resp.Warnings)))
	}

	var lastIndex uint64

	if detach || vol.State == api.HostVolumeStateReady {
		c.Ui.Output(fmt.Sprintf(
			"Created host volume %s with ID %s", vol.Name, vol.ID))
		return 0
	} else {
		c.Ui.Output(fmt.Sprintf(
			"==> Created host volume %s with ID %s", vol.Name, vol.ID))
		volID = vol.ID
		lastIndex = vol.ModifyIndex
	}

	if vol.Namespace != "" {
		client.SetNamespace(vol.Namespace)
	}

	err = c.monitorHostVolume(client, volID, lastIndex, verbose)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("==> %s: %v", formatTime(time.Now()), err.Error()))
		return 1
	}
	return 0
}

func (c *VolumeCreateCommand) monitorHostVolume(client *api.Client, id string, lastIndex uint64, verbose bool) error {
	length := shortId
	if verbose {
		length = fullId
	}

	opts := formatOpts{
		verbose: verbose,
		short:   !verbose,
		length:  length,
	}

	if isStdoutTerminal() {
		return c.ttyMonitor(client, id, lastIndex, opts)
	} else {
		return c.nottyMonitor(client, id, lastIndex, opts)
	}
}

func (c *VolumeCreateCommand) ttyMonitor(client *api.Client, id string, lastIndex uint64, opts formatOpts) error {

	gUi := glint.New()
	spinner := glint.Layout(
		components.Spinner(),
		glint.Text(fmt.Sprintf(" Monitoring volume %q in progress...", limit(id, opts.length))),
	).Row().MarginLeft(2)
	refreshRate := 100 * time.Millisecond

	gUi.SetRefreshRate(refreshRate)
	gUi.Set(spinner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gUi.Render(ctx)

	qOpts := &api.QueryOptions{
		AllowStale: true,
		WaitIndex:  lastIndex,
		WaitTime:   time.Second * 5,
	}

	var statusComponent *glint.LayoutComponent
	var endSpinner *glint.LayoutComponent

DONE:
	for {
		vol, meta, err := client.HostVolumes().Get(id, qOpts)
		if err != nil {
			return err
		}
		str, err := formatHostVolume(vol, opts)
		if err != nil {
			// should never happen b/c we don't pass json/template via opts here
			return err
		}
		statusComponent = glint.Layout(
			glint.Text(""),
			glint.Text(formatTime(time.Now())),
			glint.Text(c.Colorize().Color(str)),
		).MarginLeft(4)

		statusComponent = glint.Layout(statusComponent)
		gUi.Set(spinner, statusComponent)

		endSpinner = glint.Layout(
			components.Spinner(),
			glint.Text(fmt.Sprintf(" Host volume %q %s", limit(id, opts.length), vol.State)),
		).Row().MarginLeft(2)

		switch vol.State {
		case api.HostVolumeStateReady:
			endSpinner = glint.Layout(
				glint.Text(fmt.Sprintf("✓ Host volume %q %s", limit(id, opts.length), vol.State)),
			).Row().MarginLeft(2)
			break DONE

		case api.HostVolumeStateUnavailable:
			endSpinner = glint.Layout(
				glint.Text(fmt.Sprintf("! Host volume %q %s", limit(id, opts.length), vol.State)),
			).Row().MarginLeft(2)
			break DONE

		default:
			qOpts.WaitIndex = meta.LastIndex
			continue
		}

	}

	// Render one final time with completion message
	gUi.Set(endSpinner, statusComponent, glint.Text(""))
	gUi.RenderFrame()
	return nil
}

func (c *VolumeCreateCommand) nottyMonitor(client *api.Client, id string, lastIndex uint64, opts formatOpts) error {

	c.Ui.Info(fmt.Sprintf("==> %s: Monitoring volume %q...",
		formatTime(time.Now()), limit(id, opts.length)))

	for {
		vol, _, err := client.HostVolumes().Get(id, &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  time.Second * 5,
		})
		if err != nil {
			return err
		}
		if vol.State == api.HostVolumeStateReady {
			c.Ui.Info(fmt.Sprintf("==> %s: Volume %q ready",
				formatTime(time.Now()), limit(vol.Name, opts.length)))
			return nil
		}
	}
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
	delete(m, "capacity")
	delete(m, "capacity_max")
	delete(m, "capacity_min")
	delete(m, "type")

	// Decode the rest
	err = mapstructure.WeakDecode(m, vol)
	if err != nil {
		return nil, err
	}

	capacity, err := parseCapacityBytes(list.Filter("capacity"))
	if err != nil {
		return nil, fmt.Errorf("invalid capacity: %v", err)
	}
	vol.CapacityBytes = capacity
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
