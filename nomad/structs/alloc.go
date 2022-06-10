package structs

const (
	// AllocServiceRegistrationsRPCMethod is the RPC method for listing all
	// service registrations assigned to a specific allocation.
	//
	// Args: AllocServiceRegistrationsRequest
	// Reply: AllocServiceRegistrationsResponse
	AllocServiceRegistrationsRPCMethod = "Alloc.GetServiceRegistrations"
)

// AllocServiceRegistrationsRequest is the request object used to list all
// service registrations belonging to the specified Allocation.ID.
type AllocServiceRegistrationsRequest struct {
	AllocID string
	QueryOptions
}

// AllocServiceRegistrationsResponse is the response object when performing a
// listing of services belonging to an allocation.
type AllocServiceRegistrationsResponse struct {
	Services []*ServiceRegistration
	QueryMeta
}

// ServiceProviderNamespace returns the namespace within which the allocations
// services should be registered. This takes into account the different
// providers that can provide service registrations. In the event no services
// are found, the function will return the Consul namespace which allows hooks
// to work as they did before this feature.
//
// It currently assumes that all services within an allocation use the same
// provider and therefore the same namespace.
func (a *Allocation) ServiceProviderNamespace() string {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	if len(tg.Services) > 0 {
		switch tg.Services[0].Provider {
		case ServiceProviderNomad:
			return a.Job.Namespace
		default:
			return tg.Consul.GetNamespace()
		}
	}

	if len(tg.Tasks) > 0 {
		if len(tg.Tasks[0].Services) > 0 {
			switch tg.Tasks[0].Services[0].Provider {
			case ServiceProviderNomad:
				return a.Job.Namespace
			default:
				return tg.Consul.GetNamespace()
			}
		}
	}

	return tg.Consul.GetNamespace()
}
