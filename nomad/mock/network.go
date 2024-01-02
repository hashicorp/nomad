// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// NetworkStatus is a mock implementation of structs.NetworkStatus
type NetworkStatus struct {
	address string
}

// NewNetworkStatus creates a mock NetworkStatus based on address.
func NewNetworkStatus(address string) structs.NetworkStatus {
	return &NetworkStatus{address: address}
}

func (ns *NetworkStatus) NetworkStatus() *structs.AllocNetworkStatus {
	return &structs.AllocNetworkStatus{Address: ns.address}
}

func AllocNetworkStatus() *structs.AllocNetworkStatus {
	return &structs.AllocNetworkStatus{
		InterfaceName: "eth0",
		Address:       "192.168.0.100",
		DNS: &structs.DNSConfig{
			Servers:  []string{"1.1.1.1"},
			Searches: []string{"localdomain"},
			Options:  []string{"ndots:5"},
		},
	}
}
