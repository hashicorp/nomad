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

	// Format the allocs
	banner := c.Colorize().Color("\n[bold]Allocations[reset]")
	allocs := formatAllocListStubs(plug.Allocations, c.verbose, c.length)
	full := []string{formatKV(output), banner, allocs}
	return strings.Join(full, "\n"), nil
}
