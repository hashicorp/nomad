// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// NetworkConfigurator sets up and tears down the interfaces, routes, firewall
// rules, etc for the configured networking mode of the allocation.
type NetworkConfigurator interface {
	Setup(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error)
	Teardown(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error
}

// hostNetworkConfigurator is a noop implementation of a NetworkConfigurator for
// when the alloc join's a client host's network namespace and thus does not
// require further configuration
type hostNetworkConfigurator struct{}

func (h *hostNetworkConfigurator) Setup(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error) {
	return nil, nil
}
func (h *hostNetworkConfigurator) Teardown(context.Context, *structs.Allocation, *drivers.NetworkIsolationSpec) error {
	return nil
}

// networkingGlobalMutex is used by a synchronizedNetworkConfigurator to serialize
// network operations done by the client to prevent race conditions when manipulating
// iptables rules
var networkingGlobalMutex sync.Mutex

// synchronizedNetworkConfigurator wraps a NetworkConfigurator to provide serialized access to network
// operations performed by the client
type synchronizedNetworkConfigurator struct {
	nc NetworkConfigurator
}

func (s *synchronizedNetworkConfigurator) Setup(ctx context.Context, allocation *structs.Allocation, spec *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error) {
	networkingGlobalMutex.Lock()
	defer networkingGlobalMutex.Unlock()
	return s.nc.Setup(ctx, allocation, spec)
}

func (s *synchronizedNetworkConfigurator) Teardown(ctx context.Context, allocation *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	networkingGlobalMutex.Lock()
	defer networkingGlobalMutex.Unlock()
	return s.nc.Teardown(ctx, allocation, spec)
}
