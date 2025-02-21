// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
)

func (c *VolumeStatusCommand) csiVolumeStatus(client *api.Client, id string, opts formatOpts) error {
	if id == "" {
		return c.csiVolumesList(client, opts)
	}

	// get a CSI volume that matches the given prefix or a list of all matches if an
	// exact match is not found.
	volStub, possible, err := getByPrefix[api.CSIVolumeListStub]("volumes", client.CSIVolumes().List,
		func(vol *api.CSIVolumeListStub, prefix string) bool { return vol.ID == prefix },
		&api.QueryOptions{
			Prefix:    id,
			Namespace: c.namespace,
		})
	if err != nil {
		return fmt.Errorf("Error listing CSI volumes: %w", err)
	}
	if len(possible) > 0 {
		out, err := csiFormatVolumes(possible, c.json, c.template)
		if err != nil {
			return fmt.Errorf("Error formatting: %w", err)
		}
		return fmt.Errorf("Prefix matched multiple CSI volumes\n\n%s", out)
	}

	// Try querying the volume
	vol, _, err := client.CSIVolumes().Info(volStub.ID, nil)
	if err != nil {
		return fmt.Errorf("Error querying CSI volume: %w", err)
	}

	str, err := c.formatCSIBasic(vol)
	if err != nil {
		return fmt.Errorf("Error formatting CSI volume: %w", err)
	}
	c.Ui.Output(str)
	return nil
}

func (c *VolumeStatusCommand) csiVolumesList(client *api.Client, opts formatOpts) error {

	if !(opts.json || len(opts.template) > 0) {
		c.Ui.Output(c.Colorize().Color("[bold]Container Storage Interface[reset]"))
	}

	vols, _, err := client.CSIVolumes().List(nil)
	if err != nil {
		return fmt.Errorf("Error querying CSI volumes: %w", err)
	}

	if len(vols) == 0 {
		c.Ui.Error("No CSI volumes")
		return nil // not an empty is not an error
	} else {
		str, err := csiFormatVolumes(vols, opts.json, opts.template)
		if err != nil {
			return fmt.Errorf("Error formatting: %w", err)
		}
		c.Ui.Output(str)
	}
	if !opts.verbose {
		return nil
	}

	plugins, _, err := client.CSIPlugins().List(nil)
	if err != nil {
		return fmt.Errorf("Error querying CSI plugins: %w", err)
	}

	if len(plugins) == 0 {
		return nil // No more output if we have no plugins
	}

	var mErr *multierror.Error
	q := &api.QueryOptions{PerPage: 30} // TODO: tune page size

NEXT_PLUGIN:
	for _, plugin := range plugins {
		if !plugin.ControllerRequired || plugin.ControllersHealthy < 1 {
			continue // only controller plugins can support this query
		}
		for {
			externalList, _, err := client.CSIVolumes().ListExternal(plugin.ID, q)
			if err != nil && !errors.Is(err, io.EOF) {
				mErr = multierror.Append(mErr, fmt.Errorf(
					"Error querying CSI external volumes for plugin %q: %w", plugin.ID, err))
				// we'll stop querying this plugin, but there may be more to
				// query, so report and set the error code but move on to the
				// next plugin
				continue NEXT_PLUGIN
			}
			if externalList == nil || len(externalList.Volumes) == 0 {
				// several plugins return EOF once you hit the end of the page,
				// rather than an empty list
				continue NEXT_PLUGIN
			}
			c.Ui.Output("") // force a newline
			rows := []string{"External ID|Condition|Nodes"}
			for _, v := range externalList.Volumes {
				condition := "OK"
				if v.IsAbnormal {
					condition = fmt.Sprintf("Abnormal (%v)", v.Status)
				}
				rows = append(rows, fmt.Sprintf("%s|%s|%s",
					limit(v.ExternalID, opts.length),
					limit(condition, 20),
					strings.Join(v.PublishedExternalNodeIDs, ","),
				))
			}
			c.Ui.Output(formatList(rows))

			q.NextToken = externalList.NextToken
			if q.NextToken == "" {
				break
			}
			// we can't know the shape of arbitrarily-sized lists of volumes,
			// so break after each page
			c.Ui.Output("...")
		}
	}

	return mErr.ErrorOrNil()
}

func csiFormatVolumes(vols []*api.CSIVolumeListStub, json bool, template string) (string, error) {
	// Sort the output by volume id
	sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })

	if json || len(template) > 0 {
		out, err := Format(json, template, vols)
		if err != nil {
			return "", fmt.Errorf("format error: %w", err)
		}
		return out, nil
	}

	return csiFormatSortedVolumes(vols)
}

// Format the volumes, assumes that we're already sorted by volume ID
func csiFormatSortedVolumes(vols []*api.CSIVolumeListStub) (string, error) {
	rows := make([]string, len(vols)+1)
	rows[0] = "ID|Name|Namespace|Plugin ID|Schedulable|Access Mode"
	for i, v := range vols {
		rows[i+1] = fmt.Sprintf("%s|%s|%s|%s|%t|%s",
			v.ID,
			v.Name,
			v.Namespace,
			v.PluginID,
			v.Schedulable,
			v.AccessMode,
		)
	}
	return formatList(rows), nil
}

func (c *VolumeStatusCommand) formatCSIBasic(vol *api.CSIVolume) (string, error) {
	if c.json || len(c.template) > 0 {
		out, err := Format(c.json, c.template, vol)
		if err != nil {
			return "", fmt.Errorf("format error: %w", err)
		}
		return out, nil
	}

	output := []string{
		fmt.Sprintf("ID|%s", vol.ID),
		fmt.Sprintf("Name|%s", vol.Name),
		fmt.Sprintf("Namespace|%s", vol.Namespace),
		fmt.Sprintf("External ID|%s", vol.ExternalID),
		fmt.Sprintf("Plugin ID|%s", vol.PluginID),
		fmt.Sprintf("Provider|%s", vol.Provider),
		fmt.Sprintf("Version|%s", vol.ProviderVersion),
		fmt.Sprintf("Capacity|%s", humanize.IBytes(uint64(vol.Capacity))),
		fmt.Sprintf("Schedulable|%t", vol.Schedulable),
		fmt.Sprintf("Controllers Healthy|%d", vol.ControllersHealthy),
		fmt.Sprintf("Controllers Expected|%d", vol.ControllersExpected),
		fmt.Sprintf("Nodes Healthy|%d", vol.NodesHealthy),
		fmt.Sprintf("Nodes Expected|%d", vol.NodesExpected),
		fmt.Sprintf("Access Mode|%s", vol.AccessMode),
		fmt.Sprintf("Attachment Mode|%s", vol.AttachmentMode),
		fmt.Sprintf("Mount Options|%s", csiVolMountOption(vol.MountOptions, nil)),
	}

	// Exit early
	if c.short {
		return formatKV(output), nil
	}

	full := []string{formatKV(output)}

	if len(vol.Topologies) > 0 {
		topoBanner := c.Colorize().Color("\n[bold]Topologies[reset]")
		topo := c.formatTopology(vol)
		full = append(full, topoBanner)
		full = append(full, topo)
	}

	banner := c.Colorize().Color("\n[bold]Capabilities[reset]")
	caps := formatCSIVolumeCapabilities(vol.RequestedCapabilities)
	full = append(full, banner)
	full = append(full, caps)

	// Format the allocs
	banner = c.Colorize().Color("\n[bold]Allocations[reset]")
	allocs := formatAllocListStubs(vol.Allocations, c.verbose, c.length)
	full = append(full, banner)
	full = append(full, allocs)

	return strings.Join(full, "\n"), nil
}

func (c *VolumeStatusCommand) formatTopology(vol *api.CSIVolume) string {
	rows := []string{"Topology|Segments"}
	for i, t := range vol.Topologies {
		if t == nil {
			continue
		}
		segmentPairs := make([]string, 0, len(t.Segments))
		for k, v := range t.Segments {
			segmentPairs = append(segmentPairs, fmt.Sprintf("%s=%s", k, v))
		}
		// note: this looks awkward because we don't have any other
		// place where we list collections of arbitrary k/v's like
		// this without just dumping JSON formatted outputs. It's likely
		// the spec will expand to add extra fields, in which case we'll
		// add them here and drop the first column
		rows = append(rows, fmt.Sprintf("%02d|%v", i, strings.Join(segmentPairs, ", ")))
	}
	if len(rows) == 1 {
		return ""
	}
	return formatList(rows)
}

func csiVolMountOption(volume, request *api.CSIMountOptions) string {
	var req, opts *api.CSIMountOptions

	if request != nil {
		req = &api.CSIMountOptions{
			FSType:     request.FSType,
			MountFlags: request.MountFlags,
		}
	}

	if volume == nil {
		opts = req
	} else {
		opts = &api.CSIMountOptions{
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

func formatCSIVolumeCapabilities(caps []*api.CSIVolumeCapability) string {
	lines := make([]string, len(caps)+1)
	lines[0] = "Access Mode|Attachment Mode"
	for i, cap := range caps {
		lines[i+1] = fmt.Sprintf("%s|%s", cap.AccessMode, cap.AttachmentMode)
	}
	return formatList(lines)
}
