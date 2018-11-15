package command

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
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

func buildDeviceStatsSummaryMap(host *api.HostStats) map[string]*api.StatValue {
	r := map[string]*api.StatValue{}

	for _, dg := range host.DeviceStats {
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
