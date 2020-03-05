package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

func (c *VolumeStatusCommand) csiStatus(client *api.Client, short bool, id string) int {
	// Invoke list mode if no volume id
	if id == "" {
		c.Ui.Output(c.Colorize().Color("[bold]Container Storage Interface[reset]"))
		vols, _, err := client.CSIVolumes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying volumes: %s", err))
			return 1
		}

		if len(vols) == 0 {
			// No output if we have no volumes
			c.Ui.Output("No CSI volumes")
		} else {
			c.Ui.Output(csiFormatVolumes(vols))
		}
		return 0
	}

	// Try querying the volume
	vol, _, err := client.CSIVolumes().Info(id, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying volume: %s", err))
		return 1
	}

	c.Ui.Output(formatKV(c.formatBasic(vol)))

	// Exit early
	if short {
		return 0
	}

	// Format the allocs
	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocListStubs(vol.Allocations, c.verbose, c.length))

	return 0
}

func csiFormatVolumes(vols []*api.CSIVolumeListStub) string {
	if len(vols) == 0 {
		return "No volumes found"
	}

	// Sort the output by volume id
	sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })

	rows := make([]string, len(vols)+1)
	rows[0] = "ID|Name|Plugin ID|Schedulable|Access Mode"
	for i, v := range vols {
		rows[i+1] = fmt.Sprintf("%s|%s|%s|%t|%s",
			v.ID,
			v.Name,
			v.PluginID,
			v.Schedulable,
			v.AccessMode,
		)
	}
	return formatList(rows)
}

func (c *VolumeStatusCommand) formatBasic(vol *api.CSIVolume) []string {
	return []string{
		fmt.Sprintf("ID|%s", vol.ID),
		fmt.Sprintf("Name|%s", vol.Name),
		fmt.Sprintf("External ID|%s", vol.ExternalID),

		fmt.Sprintf("Schedulable|%t", vol.Schedulable),
		fmt.Sprintf("Controllers Healthy|%d", vol.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", vol.ControllersExpected),
		fmt.Sprintf("Nodes Healthy|%d", vol.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", vol.NodesExpected),

		fmt.Sprintf("Access Mode|%s", vol.AccessMode),
		fmt.Sprintf("Attachment Mode|%s", vol.AttachmentMode),
		fmt.Sprintf("Namespace|%s", vol.Namespace),
	}
}

func (c *VolumeStatusCommand) formatTopologies(vol *api.CSIVolume) string {
	var out []string

	// Find the union of all the keys
	head := map[string]string{}
	for _, t := range vol.Topologies {
		for key := range t.Segments {
			if _, ok := head[key]; !ok {
				head[key] = ""
			}
		}
	}

	// Append the header
	var line []string
	for key := range head {
		line = append(line, key)
	}
	out = append(out, strings.Join(line, " "))

	// Append each topology
	for _, t := range vol.Topologies {
		line = []string{}
		for key := range head {
			line = append(line, t.Segments[key])
		}
		out = append(out, strings.Join(line, " "))
	}

	return strings.Join(out, "\n")
}
