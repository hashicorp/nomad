// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// WorkloadServices describes services defined in either a Task or TaskGroup
// that need to be syncronized with a service registration provider.
type WorkloadServices struct {
	AllocInfo structs.AllocInfo

	// Canary indicates whether, or not the allocation is a canary. This is
	// used to build the correct tags mapping.
	Canary bool

	// ProviderNamespace is the provider namespace in which services will be
	// registered, if the provider supports this functionality.
	ProviderNamespace string

	// Restarter allows restarting the task or task group depending on the
	// check_restart blocks.
	Restarter WorkloadRestarter

	// Services and checks to register for the task.
	Services []*structs.Service

	// Networks from the task's resources block.
	// TODO: remove and use Ports
	Networks structs.Networks

	// NetworkStatus from alloc if network namespace is created.
	// Can be nil.
	NetworkStatus *structs.AllocNetworkStatus

	// AllocatedPorts is the list of port mappings.
	Ports structs.AllocatedPorts

	// DriverExec is the script executor for the task's driver. For group
	// services this is nil and script execution is managed by a tasklet in the
	// taskrunner script_check_hook.
	DriverExec interfaces.ScriptExecutor

	// DriverNetwork is the network specified by the driver and may be nil.
	DriverNetwork *drivers.DriverNetwork
}

// RegistrationProvider identifies the service registration provider for the
// WorkloadServices.
func (ws *WorkloadServices) RegistrationProvider() string {

	// Protect against an empty array; it would be embarrassing to panic here.
	if len(ws.Services) == 0 {
		return ""
	}

	// Note(jrasell): a Nomad task group can only currently utilise a single
	// service provider for all services included within it. In the event we
	// remove this restriction, this will need to change along which a lot of
	// other logic.
	return ws.Services[0].Provider
}

// Copy method for easing tests.
func (ws *WorkloadServices) Copy() *WorkloadServices {
	newTS := new(WorkloadServices)
	*newTS = *ws

	// Deep copy Services
	newTS.Services = make([]*structs.Service, len(ws.Services))
	for i := range ws.Services {
		newTS.Services[i] = ws.Services[i].Copy()
	}
	return newTS
}

func (ws *WorkloadServices) Name() string {
	if ws.AllocInfo.Task != "" {
		return ws.AllocInfo.Task
	}
	return "group-" + ws.AllocInfo.Group
}
