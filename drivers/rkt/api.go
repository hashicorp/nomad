package rkt

import (
	"net"
)

// This file contains the structrs used by this driver.
// Embedding structs here helps avoid depending on a linux only library

// Pod is the pod object, as defined in
// https://github.com/rkt/rkt/blob/03285a7db960311faf887452538b2b8ae4304488/api/v1/json.go#L68-L88
type Pod struct {
	UUID     string    `json:"name"`
	State    string    `json:"state"`
	Networks []NetInfo `json:"networks,omitempty"`
}

// A type and some structure to represent rkt's view of a *runtime*
// network instance.
// https://github.com/rkt/rkt/blob/4080b1743e0c46fa1645f4de64f1b75a980d82a3/networking/netinfo/netinfo.go#L29-L48
type NetInfo struct {
	NetName    string `json:"netName"`
	ConfPath   string `json:"netConf"`
	PluginPath string `json:"pluginPath"`
	IfName     string `json:"ifName"`
	IP         net.IP `json:"ip"`
	Args       string `json:"args"`
}
