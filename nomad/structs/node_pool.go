// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/exp/maps"
)

const (
	// NodePoolAll is a built-in node pool that always includes all nodes in
	// the cluster.
	NodePoolAll            = "all"
	NodePoolAllDescription = "Node pool with all nodes in the cluster."

	// NodePoolDefault is a built-in node pool for nodes that don't specify a
	// node pool in their configuration.
	NodePoolDefault            = "default"
	NodePoolDefaultDescription = "Default node pool."

	// maxNodePoolDescriptionLength is the maximum length allowed for a node
	// pool description.
	maxNodePoolDescriptionLength = 256
)

var (
	// validNodePoolName is the rule used to validate a node pool name.
	validNodePoolName = regexp.MustCompile("^[a-zA-Z0-9-_]{1,128}$")
)

// ValidadeNodePoolName returns an error if a node pool name is invalid.
func ValidateNodePoolName(pool string) error {
	if !validNodePoolName.MatchString(pool) {
		return fmt.Errorf("invalid name %q, must match regex %s", pool, validNodePoolName)
	}
	return nil
}

// NodePool allows partioning infrastructure
type NodePool struct {
	// Name is the node pool name. It must be unique.
	Name string

	// Description is the human-friendly description of the node pool.
	Description string

	// Meta is a set of user-provided metadata for the node pool.
	Meta map[string]string

	// SchedulerConfiguration is the scheduler configuration specific to the
	// node pool.
	SchedulerConfiguration *NodePoolSchedulerConfiguration

	// Raft indexes.
	CreateIndex uint64
	ModifyIndex uint64
}

// GetID implements the IDGetter interface required for pagination.
func (n *NodePool) GetID() string {
	return n.Name
}

// Validate returns an error if the node pool is invalid.
func (n *NodePool) Validate() error {
	var mErr *multierror.Error

	mErr = multierror.Append(mErr, ValidateNodePoolName(n.Name))

	if len(n.Description) > maxNodePoolDescriptionLength {
		mErr = multierror.Append(mErr, fmt.Errorf("description longer than %d", maxNodePoolDescriptionLength))
	}

	mErr = multierror.Append(mErr, n.SchedulerConfiguration.Validate())

	return mErr.ErrorOrNil()
}

// Copy returns a deep copy of the node pool.
func (n *NodePool) Copy() *NodePool {
	if n == nil {
		return nil
	}

	nc := new(NodePool)
	*nc = *n
	nc.Meta = maps.Clone(nc.Meta)
	nc.SchedulerConfiguration = nc.SchedulerConfiguration.Copy()

	return nc
}

// IsBuiltIn returns true if the node pool is one of the built-in pools.
//
// Built-in node pools are created automatically by Nomad and can never be
// deleted or modified so they are always present in the cluster..
func (n *NodePool) IsBuiltIn() bool {
	switch n.Name {
	case NodePoolAll, NodePoolDefault:
		return true
	default:
		return false
	}
}

// NodePoolSchedulerConfiguration is the scheduler confinguration applied to a
// node pool.
type NodePoolSchedulerConfiguration struct {

	// SchedulerAlgorithm is the scheduling algorithm to use for the pool.
	// If not defined, the global cluster scheduling algorithm is used.
	SchedulerAlgorithm SchedulerAlgorithm `hcl:"scheduler_algorithm"`
}

// Copy returns a deep copy of the node pool scheduler configuration.
func (n *NodePoolSchedulerConfiguration) Copy() *NodePoolSchedulerConfiguration {
	if n == nil {
		return nil
	}

	nc := new(NodePoolSchedulerConfiguration)
	*nc = *n

	return nc
}

// Validate returns an error if the node pool scheduler confinguration is
// invalid.
func (n *NodePoolSchedulerConfiguration) Validate() error {
	if n == nil {
		return nil
	}

	var mErr *multierror.Error

	switch n.SchedulerAlgorithm {
	case "", SchedulerAlgorithmBinpack, SchedulerAlgorithmSpread:
	default:
		mErr = multierror.Append(mErr, fmt.Errorf("invalid scheduler algorithm %q", n.SchedulerAlgorithm))
	}

	return mErr.ErrorOrNil()
}

// NodePoolListRequest is used to list node pools.
type NodePoolListRequest struct {
	QueryOptions
}

// NodePoolListResponse is the response to node pools list request.
type NodePoolListResponse struct {
	NodePools []*NodePool
	QueryMeta
}

// NodePoolSpecificRequest is used to make a request for a specific node pool.
type NodePoolSpecificRequest struct {
	Name string
	QueryOptions
}

// SingleNodePoolResponse is the response to a specific node pool request.
type SingleNodePoolResponse struct {
	NodePool *NodePool
	QueryMeta
}

// NodePoolUpsertRequest is used to make a request to insert or update a node
// pool.
type NodePoolUpsertRequest struct {
	NodePools []*NodePool
	WriteRequest
}

// NodePoolDeleteRequest is used to make a request to delete a node pool.
type NodePoolDeleteRequest struct {
	Names []string
	WriteRequest
}

// NodePoolNodesRequest is used to list all nodes that are part of a node pool.
type NodePoolNodesRequest struct {
	Name   string
	Fields *NodeStubFields
	QueryOptions
}

// NodePoolNodesResponse is used to return a list nodes in the node pool.
type NodePoolNodesResponse struct {
	Nodes []*NodeListStub
	QueryMeta
}

// NodePoolJobsRequest is used to make a request for the jobs in a specific node pool.
type NodePoolJobsRequest struct {
	Name   string
	Fields *JobStubFields
	QueryOptions
}

// NodePoolJobsResponse returns a list of jobs in a specific node pool.
type NodePoolJobsResponse struct {
	Jobs []*JobListStub
	QueryMeta
}
