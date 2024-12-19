// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cni

import "encoding/json"

// Conflist is the .conflist format of CNI network config.
type Conflist struct {
	CniVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Plugins    []any  `json:"plugins"`
}

// Json produces indented json of the conflist.
func (b Conflist) Json() ([]byte, error) {
	return json.MarshalIndent(b, "", "\t")
}

// NomadBridgeConfig determines the contents of the Conflist.
type NomadBridgeConfig struct {
	BridgeName     string
	AdminChainName string
	IPv4Subnet     string
	IPv6Subnet     string
	HairpinMode    bool
	ConsulCNI      bool
}

// NewNomadBridgeConflist produces a full Conflist from the config.
func NewNomadBridgeConflist(conf NomadBridgeConfig) Conflist {
	// Update website/content/docs/networking/cni.mdx when the bridge config
	// is modified. The json versions of the config can be found in
	// client/allocrunner/test_fixtures/*.conflist.json
	// If CNI plugins are added or versions need to be updated for new fields,
	// add a new constraint to nomad/job_endpoint_hooks.go

	ipRanges := [][]Range{
		{{Subnet: conf.IPv4Subnet}},
	}
	ipRoutes := []Route{
		{Dst: "0.0.0.0/0"},
	}
	if conf.IPv6Subnet != "" {
		ipRanges = append(ipRanges, []Range{{Subnet: conf.IPv6Subnet}})
		ipRoutes = append(ipRoutes, Route{Dst: "::/0"})
	}

	plugins := []any{
		Generic{
			Type: "loopback",
		},
		Bridge{
			Type:         "bridge",
			Bridgename:   conf.BridgeName,
			IpMasq:       true,
			IsGateway:    true,
			ForceAddress: true,
			HairpinMode:  conf.HairpinMode,
			Ipam: IPAM{
				Type:    "host-local",
				Ranges:  ipRanges,
				Routes:  ipRoutes,
				DataDir: "/var/run/cni",
			},
		},
		Firewall{
			Type:           "firewall",
			Backend:        "iptables",
			AdminChainName: conf.AdminChainName,
		},
		Portmap{
			Type: "portmap",
			Capabilities: PortmapCapabilities{
				Portmappings: true,
			},
			Snat: true,
		},
	}
	if conf.ConsulCNI {
		plugins = append(plugins, ConsulCNI{
			Type:     "consul-cni",
			LogLevel: "debug",
		})
	}

	return Conflist{
		CniVersion: "0.4.0",
		Name:       "nomad",
		Plugins:    plugins,
	}
}
