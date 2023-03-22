package structs

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

// RequiredNativeServiceDiscovery identifies which task groups, if any, within
// the job are utilising Nomad native service discovery.
func (j *Job) RequiredNativeServiceDiscovery() map[string]bool {
	groups := make(map[string]bool)

	for _, tg := range j.TaskGroups {

		// It is possible for services using the Nomad provider to be
		// configured at the task group level, so check here first.
		if requiresNativeServiceDiscovery(tg.Services) {
			groups[tg.Name] = true
			continue
		}

		// Iterate the tasks within the task group to check the services
		// configured at this more traditional level.
		for _, task := range tg.Tasks {
			if requiresNativeServiceDiscovery(task.Services) {
				groups[tg.Name] = true
				continue
			}
		}
	}

	return groups
}

// requiresNativeServiceDiscovery identifies whether any of the services passed
// to the function are utilising Nomad native service discovery.
func requiresNativeServiceDiscovery(services []*Service) bool {
	for _, tgService := range services {
		if tgService.Provider == ServiceProviderNomad {
			return true
		}
	}
	return false
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
