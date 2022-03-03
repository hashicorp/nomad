package api

import (
	"fmt"
	"net/url"
)

// ServiceRegistrations is used to query the service endpoints.
type ServiceRegistrations struct {
	client *Client
}

// ServiceRegistration is an instance of a single allocation advertising itself
// as a named service with a specific address. Each registration is constructed
// from the job specification Service block. Whether the service is registered
// within Nomad, and therefore generates a ServiceRegistration is controlled by
// the Service.Provider parameter.
type ServiceRegistration struct {

	// ID is the unique identifier for this registration. It currently follows
	// the Consul service registration format to provide consistency between
	// the two solutions.
	ID string

	// ServiceName is the human friendly identifier for this service
	// registration.
	ServiceName string

	// Namespace represents the namespace within which this service is
	// registered.
	Namespace string

	// NodeID is Node.ID on which this service registration is currently
	// running.
	NodeID string

	// Datacenter is the DC identifier of the node as identified by
	// Node.Datacenter.
	Datacenter string

	// JobID is Job.ID and represents the job which contained the service block
	// which resulted in this service registration.
	JobID string

	// AllocID is Allocation.ID and represents the allocation within which this
	// service is running.
	AllocID string

	// Tags are determined from either Service.Tags or Service.CanaryTags and
	// help identify this service. Tags can also be used to perform lookups of
	// services depending on their state and role.
	Tags []string

	// Address is the IP address of this service registration. This information
	// comes from the client and is not guaranteed to be routable; this depends
	// on cluster network topology.
	Address string

	// Port is the port number on which this service registration is bound. It
	// is determined by a combination of factors on the client.
	Port int

	CreateIndex uint64
	ModifyIndex uint64
}

// ServiceRegistrationListStub represents all service registrations held within a
// single namespace.
type ServiceRegistrationListStub struct {

	// Namespace details the namespace in which these services have been
	// registered.
	Namespace string

	// Services is a list of services found within the namespace.
	Services []*ServiceRegistrationStub
}

// ServiceRegistrationStub is the stub object describing an individual
// namespaced service. The object is built in a manner which would allow us to
// add additional fields in the future, if we wanted.
type ServiceRegistrationStub struct {

	// ServiceName is the human friendly name for this service as specified
	// within Service.Name.
	ServiceName string

	// Tags is a list of unique tags found for this service. The list is
	// de-duplicated automatically by Nomad.
	Tags []string
}

// ServiceRegistrations returns a new handle on the services endpoints.
func (c *Client) ServiceRegistrations() *ServiceRegistrations {
	return &ServiceRegistrations{client: c}
}

// List can be used to list all service registrations currently stored within
// the target namespace. It returns a stub response object.
func (s *ServiceRegistrations) List(q *QueryOptions) ([]*ServiceRegistrationListStub, *QueryMeta, error) {
	var resp []*ServiceRegistrationListStub
	qm, err := s.client.query("/v1/services", &resp, q)
	if err != nil {
		return nil, qm, err
	}
	return resp, qm, nil
}

// Get is used to return a list of service registrations whose name matches the
// specified parameter.
func (s *ServiceRegistrations) Get(serviceName string, q *QueryOptions) ([]*ServiceRegistration, *QueryMeta, error) {
	var resp []*ServiceRegistration
	qm, err := s.client.query("/v1/service/"+url.PathEscape(serviceName), &resp, q)
	if err != nil {
		return nil, qm, err
	}
	return resp, qm, nil
}

// Delete can be used to delete an individual service registration as defined
// by its service name and service ID.
func (s *ServiceRegistrations) Delete(serviceName, serviceID string, q *WriteOptions) (*WriteMeta, error) {
	path := fmt.Sprintf("/v1/service/%s/%s", url.PathEscape(serviceName), url.PathEscape(serviceID))
	wm, err := s.client.delete(path, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}
