// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

type Range struct {
	Subnet string `json:"subnet"`
}
type IPAM struct {
	Type   string    `json:"type"`
	Ranges [][]Range `json:"ranges"`
	Routes []Route   `json:"routes"`
}
type Loopback struct {
	Type string `json:"type"`
}
type Route struct {
	Dst string `json:"dst"`
}
type ConsulCNIBlock struct {
	Type     string `json:"type"`
	Loglevel string `json:"log-level"`
}

type Bridge struct {
	Type         string `json:"type"`
	Bridgename   string `json:"bridge"`
	IpMasq       bool   `json:"ipMasq"`
	IsGateway    bool   `json:"isGateway"`
	ForceAddress bool   `json:"forceAddress"`
	HairpinMode  bool   `json:"hairpinMode"`
	Ipam         IPAM   `json:"ipam"`
}

type Firewall struct {
	Type                   string `json:"type"`
	Backend                string `json:"backend"`
	IptablesAdminChainName string `json:"iptablesAdminChainName"`
}
type CapabilityArgs struct {
	Portmappings bool `json:"portmappings"`
}

type Portmap struct {
	Type         string         `json:"type"`
	Capabilities CapabilityArgs `json:"capabilities"`
	Snat         bool           `json:"snat"`
}

type BridgeCNIPlugin struct {
	CniVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Plugins    []any  `json:"plugins"`
}
