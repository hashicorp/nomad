package structs

import "github.com/hashicorp/nomad/helper"

// ServiceRegistration is the internal representation of a Nomad service
// registration.
type ServiceRegistration struct {

	// ID is the unique identifier for this registration. It currently follows
	// the Consul service registration format to provide consistency between
	// the two solutions.
	ID string

	// ServiceName is the human friendly identifier for this service
	// registration. This is not unique.
	ServiceName string

	// Namespace is Job.Namespace and therefore the namespace in which this
	// service registration resides.
	Namespace string

	// NodeID is Node.ID on which this service registration is currently
	// running.
	NodeID string

	// Datacenter is the DC identifier of the node as identified by
	// Node.Datacenter. It is denormalized here to allow filtering services by datacenter without looking up every node.
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

// Copy creates a deep copy of the service registration. This copy can then be
// safely modified. It handles nil objects.
func (s *ServiceRegistration) Copy() *ServiceRegistration {
	if s == nil {
		return nil
	}

	ns := new(ServiceRegistration)
	*ns = *s
	ns.Tags = helper.CopySliceString(ns.Tags)

	return ns
}

// Equals performs an equality check on the two service registrations. It
// handles nil objects.
func (s *ServiceRegistration) Equals(o *ServiceRegistration) bool {
	if s == nil || o == nil {
		return s == o
	}
	if s.ID != o.ID {
		return false
	}
	if s.ServiceName != o.ServiceName {
		return false
	}
	if s.NodeID != o.NodeID {
		return false
	}
	if s.Datacenter != o.Datacenter {
		return false
	}
	if s.JobID != o.JobID {
		return false
	}
	if s.AllocID != o.AllocID {
		return false
	}
	if s.Namespace != o.Namespace {
		return false
	}
	if s.Address != o.Address {
		return false
	}
	if s.Port != o.Port {
		return false
	}
	if !helper.CompareSliceSetString(s.Tags, o.Tags) {
		return false
	}
	return true
}
