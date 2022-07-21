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
