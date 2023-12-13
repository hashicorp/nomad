// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

func deviceQualifiedID(vendor, typ, name, id string) string {
	p := vendor
	if typ != "" {
		p += "/" + typ
	}
	if name != "" {
		p += "/" + name
	}

	return p + "[" + id + "]"
}

func buildDeviceStatsSummaryMap(deviceGroupStats []*api.DeviceGroupStats) map[string]*api.StatValue {
	r := map[string]*api.StatValue{}

	for _, dg := range deviceGroupStats {
		for id, stats := range dg.InstanceStats {
			k := deviceQualifiedID(dg.Vendor, dg.Type, dg.Name, id)
			r[k] = stats.Summary
		}
	}

	return r
}

func formatDeviceStats(qid string, stat *api.StatObject) []string {
	attrs := []string{fmt.Sprintf("Device|%s", qid)}
	formatDeviceStatsImpl(stat, "", &attrs)

	sort.Strings(attrs[1:])

	return attrs
}

func formatDeviceStatsImpl(stat *api.StatObject, keyPrefix string, result *[]string) {
	if keyPrefix != "" {
		keyPrefix = keyPrefix + "."
	}

	for n, stat := range stat.Attributes {
		*result = append(*result, fmt.Sprintf("%s%s|%s", keyPrefix, n, stat))
	}

	for k, o := range stat.Nested {
		formatDeviceStatsImpl(o, keyPrefix+k, result)
	}
}

// getDeviceResourcesForNode returns a list of devices and their statistics summary
// and tracks devices without statistics
func getDeviceResourcesForNode(deviceGroupStats []*api.DeviceGroupStats, node *api.Node) []string {
	statsSummaryMap := buildDeviceStatsSummaryMap(deviceGroupStats)

	devices := []string{}
	for _, dg := range node.NodeResources.Devices {
		for _, inst := range dg.Instances {
			id := deviceQualifiedID(dg.Vendor, dg.Type, dg.Name, inst.ID)
			statStr := ""
			if stats, ok := statsSummaryMap[id]; ok && stats != nil {
				statStr = stats.String()
			}

			devices = append(devices, fmt.Sprintf("%v|%v", id, statStr))
		}
	}

	sort.Strings(devices)

	return devices
}

// getDeviceResources returns alist of devices and their statistics summary
func getDeviceResources(deviceGroupStats []*api.DeviceGroupStats) []string {
	statsSummaryMap := buildDeviceStatsSummaryMap(deviceGroupStats)

	result := make([]string, 0, len(statsSummaryMap))
	for id, stats := range statsSummaryMap {
		result = append(result, id+"|"+stats.String())
	}

	sort.Strings(result)

	return result
}

func printDeviceStats(ui cli.Ui, deviceGroupStats []*api.DeviceGroupStats) {
	isFirst := true
	for _, dg := range deviceGroupStats {
		for id, dinst := range dg.InstanceStats {
			if !isFirst {
				ui.Output("")
			}
			isFirst = false

			qid := deviceQualifiedID(dg.Vendor, dg.Type, dg.Name, id)
			attrs := formatDeviceStats(qid, dinst.Stats)

			ui.Output(formatKV(attrs))
		}
	}
}

func getDeviceAttributes(d *api.NodeDeviceResource) []string {
	attrs := []string{fmt.Sprintf("Device Group|%s", d.ID())}

	for k, v := range d.Attributes {
		attrs = append(attrs, k+"|"+v.String())
	}

	sort.Strings(attrs[1:])

	return attrs
}
