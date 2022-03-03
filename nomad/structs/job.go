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
