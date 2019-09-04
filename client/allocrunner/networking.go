package allocrunner

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// NetworkConfigurator sets up and tears down the interfaces, routes, firewall
// rules, etc for the configured networking mode of the allocation.
type NetworkConfigurator interface {
	Setup(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error
	Teardown(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error
}

// hostNetworkConfigurator is a noop implementation of a NetworkConfigurator for
// when the alloc join's a client host's network namespace and thus does not
// require further configuration
type hostNetworkConfigurator struct{}

func (h *hostNetworkConfigurator) Setup(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error {
	return nil
}
func (h *hostNetworkConfigurator) Teardown(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error {
	return nil
}
