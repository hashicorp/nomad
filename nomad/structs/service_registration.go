// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/ipaddr"
)

const (
	// ServiceRegistrationUpsertRPCMethod is the RPC method for upserting
	// service registrations into Nomad state.
	//
	// Args: ServiceRegistrationUpsertRequest
	// Reply: ServiceRegistrationUpsertResponse
	ServiceRegistrationUpsertRPCMethod = "ServiceRegistration.Upsert"

	// ServiceRegistrationDeleteByIDRPCMethod is the RPC method for deleting
	// a service registration by its ID.
	//
	// Args: ServiceRegistrationDeleteByIDRequest
	// Reply: ServiceRegistrationDeleteByIDResponse
	ServiceRegistrationDeleteByIDRPCMethod = "ServiceRegistration.DeleteByID"

	// ServiceRegistrationListRPCMethod is the RPC method for listing service
	// registrations within Nomad.
	//
	// Args: ServiceRegistrationListRequest
	// Reply: ServiceRegistrationListResponse
	ServiceRegistrationListRPCMethod = "ServiceRegistration.List"

	// ServiceRegistrationGetServiceRPCMethod is the RPC method for detailing a
	// service and its registrations according to its name.
	//
	// Args: ServiceRegistrationByNameRequest
	// Reply: ServiceRegistrationByNameResponse
	ServiceRegistrationGetServiceRPCMethod = "ServiceRegistration.GetService"
)

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
	// Node.Datacenter. It is denormalized here to allow filtering services by
	// datacenter without looking up every node.
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
	ns.Tags = slices.Clone(ns.Tags)

	return ns
}

// Equal performs an equality check on the two service registrations. It
// handles nil objects.
func (s *ServiceRegistration) Equal(o *ServiceRegistration) bool {
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
	if !helper.SliceSetEq(s.Tags, o.Tags) {
		return false
	}
	return true
}

// Validate ensures the upserted service registration contains valid
// information and routing capabilities. Objects should never fail here as
// Nomad controls the entire registration process; but it's possible
// configuration problems could cause failures.
func (s *ServiceRegistration) Validate() error {
	if ipaddr.IsAny(s.Address) {
		return fmt.Errorf("invalid service registration address")
	}
	return nil
}

// GetID is a helper for getting the ID when the object may be nil and is
// required for pagination.
func (s *ServiceRegistration) GetID() string {
	if s == nil {
		return ""
	}
	return s.ID
}

// GetNamespace is a helper for getting the namespace when the object may be
// nil and is required for pagination.
func (s *ServiceRegistration) GetNamespace() string {
	if s == nil {
		return ""
	}
	return s.Namespace
}

// HashWith generates a unique value representative of s based on the contents of s.
func (s *ServiceRegistration) HashWith(key string) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(s.Port))

	sum := md5.New()
	sum.Write(buf)
	sum.Write([]byte(s.AllocID))
	sum.Write([]byte(s.ID))
	sum.Write([]byte(s.Namespace))
	sum.Write([]byte(s.Address))
	sum.Write([]byte(s.ServiceName))
	for _, tag := range s.Tags {
		sum.Write([]byte(tag))
	}
	sum.Write([]byte(key))
	return fmt.Sprintf("%x", sum.Sum(nil))
}

// ServiceRegistrationUpsertRequest is the request object used to upsert one or
// more service registrations.
type ServiceRegistrationUpsertRequest struct {
	Services []*ServiceRegistration
	WriteRequest
}

// ServiceRegistrationUpsertResponse is the response object when one or more
// service registrations have been successfully upserted into state.
type ServiceRegistrationUpsertResponse struct {
	WriteMeta
}

// ServiceRegistrationDeleteByIDRequest is the request object to delete a
// service registration as specified by the ID parameter.
type ServiceRegistrationDeleteByIDRequest struct {
	ID string
	WriteRequest
}

// ServiceRegistrationDeleteByIDResponse is the response object when performing a
// deletion of an individual service registration.
type ServiceRegistrationDeleteByIDResponse struct {
	WriteMeta
}

// ServiceRegistrationDeleteByNodeIDRequest is the request object to delete all
// service registrations assigned to a particular node.
type ServiceRegistrationDeleteByNodeIDRequest struct {
	NodeID string
	WriteRequest
}

// ServiceRegistrationDeleteByNodeIDResponse is the response object when
// performing a deletion of all service registrations assigned to a particular
// node.
type ServiceRegistrationDeleteByNodeIDResponse struct {
	WriteMeta
}

// ServiceRegistrationListRequest is the request object when performing service
// registration listings.
type ServiceRegistrationListRequest struct {
	QueryOptions
}

// ServiceRegistrationListResponse is the response object when performing a
// list of services. This is specifically concise to reduce the serialisation
// and network costs endpoint incur, particularly when performing blocking list
// queries.
type ServiceRegistrationListResponse struct {
	Services []*ServiceRegistrationListStub
	QueryMeta
}

// ServiceRegistrationListStub is the object which contains a list of namespace
// service registrations and their tags.
type ServiceRegistrationListStub struct {
	Namespace string
	Services  []*ServiceRegistrationStub
}

// ServiceRegistrationStub is the stub object describing an individual
// namespaced service. The object is built in a manner which would allow us to
// add additional fields in the future, if we wanted.
type ServiceRegistrationStub struct {
	ServiceName string
	Tags        []string
}

// ServiceRegistrationByNameRequest is the request object to perform a lookup
// of services matching a specific name.
type ServiceRegistrationByNameRequest struct {
	ServiceName string
	Choose      string // stable selection of n services
	QueryOptions
}

// ServiceRegistrationByNameResponse is the response object when performing a
// lookup of services matching a specific name.
type ServiceRegistrationByNameResponse struct {
	Services []*ServiceRegistration
	QueryMeta
}
