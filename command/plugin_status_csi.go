// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

func (c *PluginStatusCommand) csiBanner() {
	if !(c.json || len(c.template) > 0) {
		c.Ui.Output(c.Colorize().Color("[bold]Container Storage Interface[reset]"))
	}
}

func (c *PluginStatusCommand) csiStatus(client *api.Client, id string) int {
	if id == "" {
		c.csiBanner()
		plugs, _, err := client.CSIPlugins().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying CSI plugins: %s", err))
			return 1
		}

		if len(plugs) == 0 {
			// No output if we have no plugins
			c.Ui.Error("No CSI plugins")
		} else {
			str, err := c.csiFormatPlugins(plugs)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
				return 1
			}
			c.Ui.Output(str)
		}
		return 0
	}

	// filter by plugin if a plugin ID was passed
	plugs, _, err := client.CSIPlugins().List(&api.QueryOptions{Prefix: id})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying CSI plugins: %s", err))
		return 1
	}
	if len(plugs) == 0 {
		c.Ui.Error(fmt.Sprintf("No plugins(s) with prefix or ID %q found", id))
		return 1
	}
	if len(plugs) > 1 {
		if id != plugs[0].ID {
			out, err := c.csiFormatPlugins(plugs)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
				return 1
			}
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple plugins\n\n%s", out))
			return 1
		}
	}
	id = plugs[0].ID

	// Lookup matched a single plugin
	plug, _, err := client.CSIPlugins().Info(id, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying plugin: %s", err))
		return 1
	}

	str, err := c.csiFormatPlugin(plug)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error formatting plugin: %s", err))
		return 1
	}

	c.Ui.Output(str)
	return 0
}

func (c *PluginStatusCommand) csiFormatPlugins(plugs []*api.CSIPluginListStub) (string, error) {
	// Sort the output by quota name
	sort.Slice(plugs, func(i, j int) bool { return plugs[i].ID < plugs[j].ID })

	if c.json || len(c.template) > 0 {
		out, err := Format(c.json, c.template, plugs)
		if err != nil {
			return "", fmt.Errorf("format error: %v", err)
		}
		return out, nil
	}

	rows := make([]string, len(plugs)+1)
	rows[0] = "ID|Provider|Controllers Healthy/Expected|Nodes Healthy/Expected"
	for i, p := range plugs {
		rows[i+1] = fmt.Sprintf("%s|%s|%d/%d|%d/%d",
			limit(p.ID, c.length),
			p.Provider,
			p.ControllersHealthy,
			p.ControllersExpected,
			p.NodesHealthy,
			p.NodesExpected,
		)
	}
	return formatList(rows), nil
}

func (c *PluginStatusCommand) csiFormatPlugin(plug *api.CSIPlugin) (string, error) {
	if c.json || len(c.template) > 0 {
		out, err := Format(c.json, c.template, plug)
		if err != nil {
			return "", fmt.Errorf("format error: %v", err)
		}
		return out, nil
	}

	output := []string{
		fmt.Sprintf("ID|%s", plug.ID),
		fmt.Sprintf("Provider|%s", plug.Provider),
		fmt.Sprintf("Version|%s", plug.Version),
		fmt.Sprintf("Controllers Healthy|%d", plug.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", plug.ControllersExpected),
		fmt.Sprintf("Nodes Healthy|%d", plug.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", plug.NodesExpected),
	}

	// Exit early
	if c.short {
		return formatKV(output), nil
	}

	full := []string{formatKV(output)}

	if c.verbose {
		controllerCaps := c.formatControllerCaps(plug.Controllers)
		if controllerCaps != "" {
			full = append(full, c.Colorize().Color("\n[bold]Controller Capabilities[reset]"))
			full = append(full, controllerCaps)
		}
		nodeCaps := c.formatNodeCaps(plug.Nodes)
		if nodeCaps != "" {
			full = append(full, c.Colorize().Color("\n[bold]Node Capabilities[reset]"))
			full = append(full, nodeCaps)
		}
		topos := c.formatTopology(plug.Nodes)
		if topos != "" {
			full = append(full, c.Colorize().Color("\n[bold]Accessible Topologies[reset]"))
			full = append(full, topos)
		}

	}

	// Format the allocs
	banner := c.Colorize().Color("\n[bold]Allocations[reset]")
	allocs := formatAllocListStubs(plug.Allocations, c.verbose, c.length)
	full = append(full, banner)
	full = append(full, allocs)
	return strings.Join(full, "\n"), nil
}

func (c *PluginStatusCommand) formatControllerCaps(controllers map[string]*api.CSIInfo) string {
	caps := []string{}
	for _, controller := range controllers {
		switch info := controller.ControllerInfo; {
		case info.SupportsCreateDelete:
			caps = append(caps, "CREATE_DELETE_VOLUME")
			fallthrough
		case info.SupportsAttachDetach:
			caps = append(caps, "CONTROLLER_ATTACH_DETACH")
			fallthrough
		case info.SupportsListVolumes:
			caps = append(caps, "LIST_VOLUMES")
			fallthrough
		case info.SupportsGetCapacity:
			caps = append(caps, "GET_CAPACITY")
			fallthrough
		case info.SupportsCreateDeleteSnapshot:
			caps = append(caps, "CREATE_DELETE_SNAPSHOT")
			fallthrough
		case info.SupportsListSnapshots:
			caps = append(caps, "LIST_SNAPSHOTS")
			fallthrough
		case info.SupportsClone:
			caps = append(caps, "CLONE_VOLUME")
			fallthrough
		case info.SupportsReadOnlyAttach:
			caps = append(caps, "ATTACH_READONLY")
			fallthrough
		case info.SupportsExpand:
			caps = append(caps, "EXPAND_VOLUME")
			fallthrough
		case info.SupportsListVolumesAttachedNodes:
			caps = append(caps, "LIST_VOLUMES_PUBLISHED_NODES")
			fallthrough
		case info.SupportsCondition:
			caps = append(caps, "VOLUME_CONDITION")
			fallthrough
		case info.SupportsGet:
			caps = append(caps, "GET_VOLUME")
			fallthrough
		default:
		}
		break
	}

	if len(caps) == 0 {
		return ""
	}

	sort.StringSlice(caps).Sort()
	return "  " + strings.Join(caps, "\n  ")
}

func (c *PluginStatusCommand) formatNodeCaps(nodes map[string]*api.CSIInfo) string {
	caps := []string{}
	for _, node := range nodes {
		if node.RequiresTopologies {
			caps = append(caps, "VOLUME_ACCESSIBILITY_CONSTRAINTS")
		}
		switch info := node.NodeInfo; {
		case info.RequiresNodeStageVolume:
			caps = append(caps, "STAGE_UNSTAGE_VOLUME")
			fallthrough
		case info.SupportsStats:
			caps = append(caps, "GET_VOLUME_STATS")
			fallthrough
		case info.SupportsExpand:
			caps = append(caps, "EXPAND_VOLUME")
			fallthrough
		case info.SupportsCondition:
			caps = append(caps, "VOLUME_CONDITION")
			fallthrough
		default:
		}
		break
	}

	if len(caps) == 0 {
		return ""
	}

	sort.StringSlice(caps).Sort()
	return "  " + strings.Join(caps, "\n  ")
}

func (c *PluginStatusCommand) formatTopology(nodes map[string]*api.CSIInfo) string {
	rows := []string{"Node ID|Accessible Topology"}
	for nodeID, node := range nodes {
		if node.NodeInfo.AccessibleTopology != nil {
			segments := node.NodeInfo.AccessibleTopology.Segments
			segmentPairs := make([]string, 0, len(segments))
			for k, v := range segments {
				segmentPairs = append(segmentPairs, fmt.Sprintf("%s=%s", k, v))
			}
			rows = append(rows, fmt.Sprintf("%s|%s", nodeID[:8], strings.Join(segmentPairs, ",")))
		}
	}
	if len(rows) == 1 {
		return ""
	}
	return formatList(rows)
}
