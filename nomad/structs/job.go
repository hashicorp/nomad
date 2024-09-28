// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/hashicorp/go-set/v3"
)

const (
	// JobBatchDeregisterRPCMethod is the RPC method for batch removing jobs
	// from Nomad state. This is not exposed externally and is used by Nomads
	// internal GC.
	//
	// Args: JobBatchDeregisterRequest
	// Reply: JobBatchDeregisterResponse
	JobBatchDeregisterRPCMethod = "Job.BatchDeregister"

	// JobServiceRegistrationsRPCMethod is the RPC method for listing all
	// service registrations assigned to a specific namespaced job.
	//
	// Args: JobServiceRegistrationsRequest
	// Reply: JobServiceRegistrationsResponse
	JobServiceRegistrationsRPCMethod = "Job.GetServiceRegistrations"
)

// JobBatchDeregisterRequest is used to batch deregister jobs and upsert
// evaluations.
type JobBatchDeregisterRequest struct {

	// Jobs is the set of jobs to deregister.
	Jobs map[NamespacedID]*JobDeregisterOptions

	// SubmitTime is the time at which the job was requested to be stopped.
	//
	// Deprecated: The job batch deregister endpoint is only used by internal
	// garbage collection meaning the job is removed from state, and we do not
	// need to modify the submit time.
	SubmitTime int64

	WriteRequest
}

// JobDeregisterOptions configures how a job is deregistered.
type JobDeregisterOptions struct {

	// Purge controls whether the deregister purges the job from the system or
	// whether the job is just marked as stopped and will be removed by the
	// garbage collector.
	//
	// This request option is only ever used by the internal garbage collection
	// process, so is always set to true.
	Purge bool
}

// JobBatchDeregisterResponse is used to respond to a batch job deregistration.
type JobBatchDeregisterResponse struct {
	QueryMeta
}

// JobStatusesRequest is used on the Job.Statuses RPC endpoint
// to get job/alloc/deployment status for jobs.
type JobStatusesRequest struct {
	// Jobs may be optionally provided to request a subset of specific jobs.
	Jobs []NamespacedID
	// IncludeChildren will include child (batch) jobs in the response.
	IncludeChildren bool
	QueryOptions
}

// JobStatusesResponse is the response from Job.Statuses RPC endpoint.
type JobStatusesResponse struct {
	Jobs []JobStatusesJob
	QueryMeta
}

// JobStatusesJob collates information about a Job, its Allocation(s),
// and latest Deployment.
type JobStatusesJob struct {
	NamespacedID
	Name        string
	Type        string
	NodePool    string
	Datacenters []string
	Priority    int
	Version     uint64
	SubmitTime  int64
	ModifyIndex uint64
	// Allocs contains information about current allocations
	Allocs []JobStatusesAlloc
	// GroupCountSum is the sum of all group{count=X} values,
	// can be compared against number of running allocs to determine
	// overall health for "service" jobs.
	GroupCountSum int
	// ChildStatuses contains the statuses of child (batch) jobs
	ChildStatuses []string
	// ParentID is set on child (batch) jobs, specifying the parent job ID
	ParentID         string
	LatestDeployment *JobStatusesLatestDeployment
	Stop             bool // has the job been manually stopped?
	IsPack           bool // is pack metadata present?
	Status           string
}

// JobStatusesAlloc contains a subset of Allocation info.
type JobStatusesAlloc struct {
	ID               string
	Group            string
	ClientStatus     string
	NodeID           string
	DeploymentStatus JobStatusesDeployment
	JobVersion       uint64
	FollowupEvalID   string
	// HasPausedTask is true if any of the tasks in the allocation
	// are Paused (Enterprise)
	HasPausedTask bool
}

// JobStatusesDeployment contains a subset of AllocDeploymentStatus info.
type JobStatusesDeployment struct {
	Canary  bool
	Healthy *bool
}

// JobStatusesLatestDeployment contains a subset of the latest Deployment.
type JobStatusesLatestDeployment struct {
	ID                string
	IsActive          bool
	JobVersion        uint64
	Status            string
	StatusDescription string
	AllAutoPromote    bool
	RequiresPromotion bool
}

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
		if tgService.IsConsul() {
			return true
		}
	}
	return false
}

// RequiredNUMA identifies which task groups, if any, within the job contain
// tasks requesting NUMA resources.
func (j *Job) RequiredNUMA() set.Collection[string] {
	result := set.New[string](10)
	for _, tg := range j.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Resources != nil && task.Resources.NUMA.Requested() {
				result.Insert(tg.Name)
				break
			}
		}
	}
	return result
}

// RequiredBridgeNetwork identifies which task groups, if any, within the job
// contain networks requesting bridge networking.
func (j *Job) RequiredBridgeNetwork() set.Collection[string] {
	result := set.New[string](len(j.TaskGroups))
	for _, tg := range j.TaskGroups {
		if tg.Networks.Modes().Contains("bridge") {
			result.Insert(tg.Name)
		}
	}
	return result
}

// RequiredTransparentProxy identifies which task groups, if any, within the job
// contain Connect blocks using transparent proxy
func (j *Job) RequiredTransparentProxy() set.Collection[string] {
	result := set.New[string](len(j.TaskGroups))
	for _, tg := range j.TaskGroups {
		for _, service := range tg.Services {
			if service.Connect != nil {
				if service.Connect.HasTransparentProxy() {
					result.Insert(tg.Name)
					break // to next TaskGroup
				}
			}
		}
	}

	return result
}

// RequiredScheduleTask collects any groups within the job that have
// tasks with a schedule{} block for time based task execution (Enterprise)
func (j *Job) RequiredScheduleTask() set.Collection[string] {
	result := set.New[string](len(j.TaskGroups))
	for _, tg := range j.TaskGroups {
		for _, t := range tg.Tasks {
			if t.Schedule != nil {
				result.Insert(tg.Name)
				break // to next TaskGroup
			}
		}
	}
	return result
}
