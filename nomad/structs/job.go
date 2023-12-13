// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/hashicorp/go-set"
)

const (
	// JobServiceRegistrationsRPCMethod is the RPC method for listing all
	// service registrations assigned to a specific namespaced job.
	//
	// Args: JobServiceRegistrationsRequest
	// Reply: JobServiceRegistrationsResponse
	JobServiceRegistrationsRPCMethod = "Job.GetServiceRegistrations"
)

// JobServiceRegistrationsRequest is the request object used to list all
// service registrations belonging to the specified Job.ID.
type JobServiceRegistrationsRequest struct {
	JobID string
	QueryOptions
}

// JobServiceRegistrationsResponse is the response object when performing a
// listing of services belonging to a namespaced job.
type JobServiceRegistrationsResponse struct {
	Services []*ServiceRegistration
	QueryMeta
}

// NativeServiceDiscoveryUsage tracks which groups make use of the nomad service
// discovery provider, and also which of those groups make use of checks. This
// information will be used to configure implicit constraints on the job.
type NativeServiceDiscoveryUsage struct {
	Basic  *set.Set[string] // implies v1.3.0 + ${attr.nomad.service_discovery}
	Checks *set.Set[string] // implies v1.4.0
}

// Empty returns true if no groups are using native service discovery.
func (u *NativeServiceDiscoveryUsage) Empty() bool {
	return u.Basic.Size() == 0 && u.Checks.Size() == 0
}

// RequiredNativeServiceDiscovery identifies which task groups, if any, within
// the job are utilising Nomad native service discovery.
func (j *Job) RequiredNativeServiceDiscovery() *NativeServiceDiscoveryUsage {
	basic := set.New[string](10)
	checks := set.New[string](10)

	for _, tg := range j.TaskGroups {
		// It is possible for services using the Nomad provider to be
		// configured at the group level, so check here first.
		requiresNativeServiceDiscovery(tg.Name, tg.Services, basic, checks)

		// Iterate the tasks within the task group to check the services
		// configured at this more traditional level.
		for _, task := range tg.Tasks {
			requiresNativeServiceDiscovery(tg.Name, task.Services, basic, checks)
		}
	}
	return &NativeServiceDiscoveryUsage{
		Basic:  basic,
		Checks: checks,
	}
}

// requiresNativeServiceDiscovery identifies whether any of the services passed
// to the function are utilising Nomad native service discovery.
func requiresNativeServiceDiscovery(group string, services []*Service, basic, checks *set.Set[string]) {
	for _, tgService := range services {
		if tgService.Provider == ServiceProviderNomad {
			basic.Insert(group)
			if len(tgService.Checks) > 0 {
				checks.Insert(group)
			}
		}
	}
}

// RequiredConsulServiceDiscovery identifies which task groups, if any, within
// the job are utilising Consul service discovery.
func (j *Job) RequiredConsulServiceDiscovery() map[string]bool {
	groups := make(map[string]bool)

	for _, tg := range j.TaskGroups {

		// It is possible for services using the Consul provider to be
		// configured at the task group level, so check here first. This is
		// a requirement for Consul Connect services.
		if requiresConsulServiceDiscovery(tg.Services) {
			groups[tg.Name] = true
			continue
		}

		// Iterate the tasks within the task group to check the services
		// configured at this more traditional level.
		for _, task := range tg.Tasks {
			if requiresConsulServiceDiscovery(task.Services) {
				groups[tg.Name] = true
				continue
			}
		}
	}

	return groups
}

// requiresConsulServiceDiscovery identifies whether any of the services passed
// to the function are utilising Consul service discovery.
func requiresConsulServiceDiscovery(services []*Service) bool {
	for _, tgService := range services {
		if tgService.Provider == ServiceProviderConsul || tgService.Provider == "" {
			return true
		}
	}
	return false
}
