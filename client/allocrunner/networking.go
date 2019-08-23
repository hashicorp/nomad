package allocrunner

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// NetworkConfigurator sets up and tears down the interfaces, routes, firewall
// rules, etc for the configured networking mode of the allocation.
type NetworkConfigurator interface {
	Init() error
	Setup(*structs.Allocation, *drivers.NetworkIsolationSpec) error
	Teardown(*structs.Allocation, *drivers.NetworkIsolationSpec) error
}

// hostNetworkConfigurator is a noop implementation of a NetworkConfigurator for
// when the alloc join's a client host's network namespace and thus does not
// require further configuration
type hostNetworkConfigurator struct{}

func (h *hostNetworkConfigurator) Init() error { return nil }

func (h *hostNetworkConfigurator) Setup(*structs.Allocation, *drivers.NetworkIsolationSpec) error {
	return nil
}
func (h *hostNetworkConfigurator) Teardown(*structs.Allocation, *drivers.NetworkIsolationSpec) error {
	return nil
}
