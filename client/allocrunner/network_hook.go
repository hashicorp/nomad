package allocrunner

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// networkHook is an alloc lifecycle hook that manages the network namespace
// for an alloc
type networkHook struct {
	// setter is a callback to set the network isolation spec when after the
	// network is created
	setter networkIsolationSetter

	// manager is used when creating the network namespace. This defaults to
	// bind mounting a network namespace descritor under /var/run/netns but
	// can be created by a driver if nessicary
	manager drivers.DriverNetworkManager

	// alloc should only be read from
	alloc *structs.Allocation

	// spec described the network namespace and is syncronized by specLock
	spec *drivers.NetworkIsolationSpec

	// networkConfigurator configures the network interfaces, routes, etc once
	// the alloc network has been created
	networkConfigurator NetworkConfigurator

	logger hclog.Logger
}

func newNetworkHook(logger hclog.Logger, ns networkIsolationSetter,
	alloc *structs.Allocation, netManager drivers.DriverNetworkManager,
	netConfigurator NetworkConfigurator) *networkHook {
	return &networkHook{
		setter:              ns,
		alloc:               alloc,
		manager:             netManager,
		networkConfigurator: netConfigurator,
		logger:              logger,
	}
}

func (h *networkHook) Name() string {
	return "network"
}

func (h *networkHook) Prerun() error {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	if len(tg.Networks) == 0 || tg.Networks[0].Mode == "host" || tg.Networks[0].Mode == "" {
		return nil
	}

	if h.manager == nil || h.networkConfigurator == nil {
		h.logger.Trace("shared network namespaces are not supported on this platform, skipping network hook")
		return nil
	}

	spec, created, err := h.manager.CreateNetwork(h.alloc.ID)

	if err != nil {
		return fmt.Errorf("failed to create network for alloc: %v", err)
	}

	if spec != nil {
		h.spec = spec
		h.setter.SetNetworkIsolation(spec)
	}

	if created {
		if err := h.networkConfigurator.Setup(context.TODO(), h.alloc, spec); err != nil {
			return fmt.Errorf("failed to configure networking for alloc: %v", err)
		}
	}
	return nil
}

func (h *networkHook) Postrun() error {
	if h.spec == nil {
		return nil
	}

	if err := h.networkConfigurator.Teardown(context.TODO(), h.alloc, h.spec); err != nil {
		h.logger.Error("failed to cleanup network for allocation, resources may have leaked", "alloc", h.alloc.ID, "error", err)
	}
	return h.manager.DestroyNetwork(h.alloc.ID, h.spec)
}
