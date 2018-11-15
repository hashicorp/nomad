package command

import (
	"fmt"

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

func formatDeviceStats(stat *api.StatObject, keyPrefix string, result *[]string) {
	if keyPrefix != "" {
		keyPrefix = keyPrefix + "."
	}

	for n, stat := range stat.Attributes {
		*result = append(*result, fmt.Sprintf("%s%s|%s", keyPrefix, n, stat))
	}

	for k, o := range stat.Nested {
		formatDeviceStats(o, keyPrefix+k, result)
	}
}

// getDeviceResources returns a list of devices and their statistics summary
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

	return devices
}

func printDeviceStats(ui cli.Ui, deviceGroupStats []*api.DeviceGroupStats) {
	isFirst := true
	for _, dg := range deviceGroupStats {
		for id, dinst := range dg.InstanceStats {
			if !isFirst {
				ui.Output("\n")
			}
			isFirst = false

			qid := deviceQualifiedID(dg.Vendor, dg.Type, dg.Name, id)
			attrs := make([]string, 1, len(dinst.Stats.Attributes)+1)
			attrs[0] = fmt.Sprintf("Device|%s", qid)
			formatDeviceStats(dinst.Stats, "", &attrs)

			ui.Output(formatKV(attrs))
		}
	}
}
