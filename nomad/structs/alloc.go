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
