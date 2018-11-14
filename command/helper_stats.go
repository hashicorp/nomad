package command

import (
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
