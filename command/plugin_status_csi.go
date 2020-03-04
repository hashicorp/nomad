package command

import (
	"fmt"
	"sort"

	"github.com/hashicorp/nomad/api"
)

func (c *CSIPluginStatusCommand) csiStatus(client *api.Client, short bool, id string) int {
	if id == "" {
		plugs, _, err := client.CSIPlugins().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying CSI plugins: %s", err))
			return 1
		}

		if len(plugs) == 0 {
			// No output if we have no jobs
			c.Ui.Output("No CSI plugins")
		} else {
			c.Ui.Output(formatCSIPluginList(plugs))
		}
		return 0
	}

	// Lookup matched a single job
	plug, _, err := client.CSIPlugins().Info(id, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying plugin: %s", err))
		return 1
	}

	c.Ui.Output(formatKV(c.formatBasic(plug)))

	// Exit early
	if short {
		return 0
	}

	// Format the allocs
	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocListStubs(plug.Allocations, c.verbose, c.length))

	return 0
}

func (c *PluginStatusCommand) csiFormatPlugins(plugs []*api.CSIPluginListStub) string {
	if len(plugs) == 0 {
		return "No plugins found"
	}

	// Sort the output by quota name
	sort.Slice(plugs, func(i, j int) bool { return plugs[i].ID < plugs[j].ID })

	rows := make([]string, len(plugs)+1)
	rows[0] = "ID|Controllers Healthy|Controllers Expected|Nodes Healthy|Nodes Expected"
	for i, p := range plugs {
		rows[i+1] = fmt.Sprintf("%s|%d|%d|%d|%d",
			p.ID,
			p.ControllersHealthy,
			p.ControllersExpected,
			p.NodesHealthy,
			p.NodesExpected,
		)
	}
	return formatList(rows)
}

func (v *PluginStatusCommand) csiFormatPlugin(plug *api.CSIPlugin) []string {
	output := []string{
		fmt.Sprintf("ID|%s", plug.ID),
		fmt.Sprintf("Controllers Healthy|%d", plug.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", len(plug.Controllers)),
		fmt.Sprintf("Nodes Healthy|%d", plug.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", len(plug.Nodes)),
	}

	return output
}
