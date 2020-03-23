package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (c *VolumeStatusCommand) csiBanner() {
	if !(c.json || len(c.template) > 0) {
		c.Ui.Output(c.Colorize().Color("[bold]Container Storage Interface[reset]"))
	}
}

func (c *VolumeStatusCommand) csiStatus(client *api.Client, id string) int {
	// Invoke list mode if no volume id
	if id == "" {
		c.csiBanner()
		vols, _, err := client.CSIVolumes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying volumes: %s", err))
			return 1
		}

		if len(vols) == 0 {
			// No output if we have no volumes
			c.Ui.Error("No CSI volumes")
		} else {
			str, err := c.csiFormatVolumes(vols)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error formatting: %s", err))
				return 1
			}
			c.Ui.Output(str)
		}
		return 0
	}

	// Try querying the volume
	vol, _, err := client.CSIVolumes().Info(id, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying volume: %s", err))
		return 1
	}

	str, err := c.formatBasic(vol)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error formatting volume: %s", err))
		return 1
	}
	c.Ui.Output(str)

	return 0
}

func (c *VolumeStatusCommand) csiFormatVolumes(vols []*api.CSIVolumeListStub) (string, error) {
	// Sort the output by volume id
	sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })

	if c.json || len(c.template) > 0 {
		out, err := Format(c.json, c.template, vols)
		if err != nil {
			return "", fmt.Errorf("format error: %v", err)
		}
		return out, nil
	}

	rows := make([]string, len(vols)+1)
	rows[0] = "ID|Name|Plugin ID|Schedulable|Access Mode"
	for i, v := range vols {
		rows[i+1] = fmt.Sprintf("%s|%s|%s|%t|%s",
			limit(v.ID, c.length),
			v.Name,
			v.PluginID,
			v.Schedulable,
			v.AccessMode,
		)
	}
	return formatList(rows), nil
}

func (c *VolumeStatusCommand) formatBasic(vol *api.CSIVolume) (string, error) {
	if c.json || len(c.template) > 0 {
		out, err := Format(c.json, c.template, vol)
		if err != nil {
			return "", fmt.Errorf("format error: %v", err)
		}
		return out, nil
	}

	output := []string{
		fmt.Sprintf("ID|%s", vol.ID),
		fmt.Sprintf("Name|%s", vol.Name),
		fmt.Sprintf("External ID|%s", vol.ExternalID),
		fmt.Sprintf("Plugin ID|%s", vol.PluginID),
		fmt.Sprintf("Provider|%s", vol.Provider),
		fmt.Sprintf("Version|%s", vol.ProviderVersion),
		fmt.Sprintf("Schedulable|%t", vol.Schedulable),
		fmt.Sprintf("Controllers Healthy|%d", vol.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", vol.ControllersExpected),
		fmt.Sprintf("Nodes Healthy|%d", vol.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", vol.NodesExpected),

		fmt.Sprintf("Access Mode|%s", vol.AccessMode),
		fmt.Sprintf("Attachment Mode|%s", vol.AttachmentMode),
		fmt.Sprintf("Mount Options|%s", csiVolMountOption(vol.MountOptions, nil)),
		fmt.Sprintf("Namespace|%s", vol.Namespace),
	}

	// Exit early
	if c.short {
		return formatKV(output), nil
	}

	// Format the allocs
	banner := c.Colorize().Color("\n[bold]Allocations[reset]")
	allocs := formatAllocListStubs(vol.Allocations, c.verbose, c.length)
	full := []string{formatKV(output), banner, allocs}
	return strings.Join(full, "\n"), nil
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

func csiVolMountOption(volume, request *api.CSIMountOptions) string {
	var req, opts *structs.CSIMountOptions

	if request != nil {
		req = &structs.CSIMountOptions{
			FSType:     request.FSType,
			MountFlags: request.MountFlags,
		}
	}

	if volume == nil {
		opts = req
	} else {
		opts = &structs.CSIMountOptions{
			FSType:     volume.FSType,
			MountFlags: volume.MountFlags,
		}
		opts.Merge(req)
	}

	if opts == nil {
		return "<none>"
	}

	var out string
	if opts.FSType != "" {
		out = fmt.Sprintf("fs_type: %s", opts.FSType)
	}

	if len(opts.MountFlags) > 0 {
		out = fmt.Sprintf("%s flags: %s", out, strings.Join(opts.MountFlags, ", "))
	}

	return out
}
