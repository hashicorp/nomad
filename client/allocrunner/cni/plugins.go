// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cni

// Generic has the one key that all plugins must have: "type"
type Generic struct {
	Type string `json:"type"`
}

// Bridge is the subset of options that we use to configure the "bridge" plugin.
// https://www.cni.dev/plugins/current/main/bridge/
type Bridge struct {
	Type         string `json:"type"`
	Bridgename   string `json:"bridge"`
	IpMasq       bool   `json:"ipMasq"`
	IsGateway    bool   `json:"isGateway"`
	ForceAddress bool   `json:"forceAddress"`
	HairpinMode  bool   `json:"hairpinMode"`
	Ipam         IPAM   `json:"ipam"`
}
type IPAM struct {
	Type    string    `json:"type"`
	Ranges  [][]Range `json:"ranges"`
	Routes  []Route   `json:"routes"`
	DataDir string    `json:"dataDir"`
}
type Range struct {
	Subnet string `json:"subnet"`
}
type Route struct {
	Dst string `json:"dst"`
}

// Firewall is the "firewall" plugin.
// https://www.cni.dev/plugins/current/meta/firewall/
type Firewall struct {
	Type           string `json:"type"`
	Backend        string `json:"backend"`
	AdminChainName string `json:"iptablesAdminChainName"`
}

// Portmap is the "portmap" plugin.
// https://www.cni.dev/plugins/current/meta/portmap/
type Portmap struct {
	Type         string              `json:"type"`
	Capabilities PortmapCapabilities `json:"capabilities"`
	Snat         bool                `json:"snat"`
}
type PortmapCapabilities struct {
	Portmappings bool `json:"portMappings"`
}

// ConsulCNI is the "consul-cni" plugin used for transparent proxy.
// https://github.com/hashicorp/consul-k8s/blob/main/control-plane/cni/main.go
type ConsulCNI struct {
	Type     string `json:"type"`
	LogLevel string `json:"log_level"`
}
