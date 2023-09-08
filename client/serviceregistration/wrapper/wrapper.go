// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package wrapper

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/nomad/structs"
)

// HandlerWrapper is used to wrap service registration implementations of the
// Handler interface. We do not use a map or similar to store the handlers, so
// we can avoid having to use a lock. This may need to be updated if we ever
// support additional registration providers.
type HandlerWrapper struct {
	log hclog.Logger

	// consulServiceProvider is the handler for services where Consul is the
	// provider. This provider is always created and available.
	consulServiceProvider serviceregistration.Handler

	// nomadServiceProvider is the handler for services where Nomad is the
	// provider.
	nomadServiceProvider serviceregistration.Handler
}

// NewHandlerWrapper configures and returns a HandlerWrapper for use within
// client hooks that need to interact with service and check registrations. It
// mimics the serviceregistration.Handler interface, but returns the
// implementation to allow future flexibility and is initially only intended
// for use with the alloc and task runner service hooks.
func NewHandlerWrapper(
	log hclog.Logger, consulProvider, nomadProvider serviceregistration.Handler) *HandlerWrapper {
	return &HandlerWrapper{
		log:                   log,
		nomadServiceProvider:  nomadProvider,
		consulServiceProvider: consulProvider,
	}
}

// RegisterWorkload wraps the serviceregistration.Handler RegisterWorkload
// function. It determines which backend provider to call and passes the
// workload unless the provider is unknown, in which case an error will be
// returned.
func (h *HandlerWrapper) RegisterWorkload(workload *serviceregistration.WorkloadServices) error {

	// Don't rely on callers to check there are no services to register.
	if len(workload.Services) == 0 {
		return nil
	}

	provider := workload.RegistrationProvider()

	switch provider {
	case structs.ServiceProviderNomad:
		return h.nomadServiceProvider.RegisterWorkload(workload)
	case structs.ServiceProviderConsul:
		return h.consulServiceProvider.RegisterWorkload(workload)
	default:
		return fmt.Errorf("unknown service registration provider: %q", provider)
	}
}

// RemoveWorkload wraps the serviceregistration.Handler RemoveWorkload
// function. It determines which backend provider to call and passes the
// workload unless the provider is unknown.
func (h *HandlerWrapper) RemoveWorkload(services *serviceregistration.WorkloadServices) {

	var provider string

	// It is possible the services field is empty depending on the exact
	// situation which resulted in the call.
	if len(services.Services) > 0 {
		provider = services.RegistrationProvider()
	}

	// Call the correct provider, if we have managed to identify it. An empty
	// string means you didn't find a provider, therefore default to consul.
	//
	// In certain situations this function is called with zero services,
	// therefore meaning we make an assumption on the provider. When this
	// happens, we need to ensure the allocation is removed from the Consul
	// implementation. This tracking (allocRegistrations) is used by the
	// allochealth tracker and so is critical to be removed. The test
	// allocrunner.TestAllocRunner_Restore_RunningTerminal covers the case
	// described here.
	switch provider {
	case structs.ServiceProviderNomad:
		h.nomadServiceProvider.RemoveWorkload(services)
	case structs.ServiceProviderConsul, "":
		h.consulServiceProvider.RemoveWorkload(services)
	default:
		h.log.Error("unknown service registration provider", "provider", provider)
	}
}

// UpdateWorkload identifies which provider to call for the new and old
// workloads provided. In the event both use the same provider, the
// UpdateWorkload function will be called, otherwise the register and remove
// functions will be called.
func (h *HandlerWrapper) UpdateWorkload(old, new *serviceregistration.WorkloadServices) error {

	// Hot path to exit if there is nothing to do.
	if len(old.Services) == 0 && len(new.Services) == 0 {
		return nil
	}

	newProvider := new.RegistrationProvider()
	oldProvider := old.RegistrationProvider()

	// If the new and old services use the same provider, call the
	// UpdateWorkload and leave it at that.
	if newProvider == oldProvider {
		switch newProvider {
		case structs.ServiceProviderNomad:
			return h.nomadServiceProvider.UpdateWorkload(old, new)
		case structs.ServiceProviderConsul:
			return h.consulServiceProvider.UpdateWorkload(old, new)
		default:
			return fmt.Errorf("unknown service registration provider for update: %q", newProvider)
		}
	}

	// If we have new services, call the relevant provider. Registering can
	// return an error. Do this before RemoveWorkload, so we can halt the
	// process if needed, otherwise we may leave the task/group
	// registration-less.
	if len(new.Services) > 0 {
		if err := h.RegisterWorkload(new); err != nil {
			return err
		}
	}

	if len(old.Services) > 0 {
		h.RemoveWorkload(old)
	}

	return nil
}
