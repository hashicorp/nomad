// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"context"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/maps"
)

// Handler is the interface the Nomad Client uses to register, update and
// remove services and checks from service registration providers. Currently,
// Consul and Nomad are supported providers.
//
// When utilising Consul, the ACL "service:write" is required. It supports all
// functionality and is the OG/GOAT.
//
// When utilising Nomad, the client secret ID is used for authorisation. It
// currently supports service registrations only.
type Handler interface {

	// RegisterWorkload adds all service entries and checks to the backend
	// provider. Whilst callers attempt to ensure WorkloadServices.Services is
	// not empty before calling this function, implementations should also
	// perform this.
	RegisterWorkload(workload *WorkloadServices) error

	// RemoveWorkload all service entries and checks from the backend provider
	// that are found within the passed WorkloadServices object. Whilst callers
	// attempt to ensure WorkloadServices.Services is not empty before calling
	// this function, implementations should also perform this.
	RemoveWorkload(workload *WorkloadServices)

	// UpdateWorkload removes workload as specified by the old parameter, and
	// adds workload as specified by the new parameter. Callers do not perform
	// any deduplication on both objects, it is therefore the responsibility of
	// the implementation.
	UpdateWorkload(old, newTask *WorkloadServices) error

	// AllocRegistrations returns the registrations for the given allocation.
	AllocRegistrations(allocID string) (*AllocRegistration, error)

	// UpdateTTL is used to update the TTL of an individual service
	// registration check.
	UpdateTTL(id, namespace, output, status string) error
}

// WorkloadRestarter allows the checkWatcher to restart tasks or entire task
// groups.
type WorkloadRestarter interface {
	Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error
}

// AllocRegistration holds the status of services registered for a particular
// allocations by task.
type AllocRegistration struct {
	// Tasks maps the name of a task to its registered services and checks.
	Tasks map[string]*ServiceRegistrations
}

// Copy performs a deep copy of the AllocRegistration object.
func (a *AllocRegistration) Copy() *AllocRegistration {
	c := &AllocRegistration{
		Tasks: make(map[string]*ServiceRegistrations, len(a.Tasks)),
	}

	for k, v := range a.Tasks {
		c.Tasks[k] = v.copy()
	}

	return c
}

// NumServices returns the number of registered task AND group services.
// Group services are prefixed with "group-".
func (a *AllocRegistration) NumServices() int {
	if a == nil {
		return 0
	}

	total := 0
	for _, task := range a.Tasks {
		total += len(task.Services)
	}

	return total
}

// NumChecks returns the number of registered checks from both task AND group
// services. Group services are prefixed with "group-".
func (a *AllocRegistration) NumChecks() int {
	if a == nil {
		return 0
	}

	total := 0
	for _, task := range a.Tasks {
		for _, service := range task.Services {
			total += len(service.Checks)
		}
	}

	return total
}

// ServiceRegistrations holds the status of services registered for a
// particular task or task group.
type ServiceRegistrations struct {
	// Services maps service_id -> service registration
	Services map[string]*ServiceRegistration
}

func (t *ServiceRegistrations) copy() *ServiceRegistrations {
	c := &ServiceRegistrations{
		Services: make(map[string]*ServiceRegistration, len(t.Services)),
	}

	for k, v := range t.Services {
		c.Services[k] = v.copy()
	}

	return c
}

// ServiceRegistration holds the status of a registered Consul Service and its
// Checks.
type ServiceRegistration struct {
	// serviceID and checkIDs are internal fields that track just the IDs of the
	// services/checks registered in Consul. It is used to materialize the other
	// fields when queried.
	ServiceID string
	CheckIDs  map[string]struct{} // todo: use a Set?

	// CheckOnUpdate is a map of checkIDs and the associated OnUpdate value
	// from the ServiceCheck It is used to determine how a reported checks
	// status should be evaluated.
	CheckOnUpdate map[string]string

	// Service is the AgentService registered in Consul.
	Service *api.AgentService

	// Checks is the status of the registered checks.
	Checks []*api.AgentCheck
}

func (s *ServiceRegistration) copy() *ServiceRegistration {
	// Copy does not copy the external fields but only the internal fields.
	// This is so that the caller of AllocRegistrations can not access the
	// internal fields and that method uses these fields to populate the
	// external fields.
	return &ServiceRegistration{
		ServiceID:     s.ServiceID,
		CheckIDs:      maps.Clone(s.CheckIDs),
		CheckOnUpdate: maps.Clone(s.CheckOnUpdate),
	}
}
