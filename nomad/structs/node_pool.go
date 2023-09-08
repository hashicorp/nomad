// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/pointer"
	"golang.org/x/crypto/blake2b"
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

// ValidateNodePoolName returns an error if a node pool name is invalid.
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

	// Hash is the hash of the node pool which is used to efficiently diff when
	// we replicate pools across regions.
	Hash []byte

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

	nc.Hash = make([]byte, len(n.Hash))
	copy(nc.Hash, n.Hash)

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

// MemoryOversubscriptionEnabled returns true if memory oversubscription is
// enabled in the node pool or in the global cluster configuration.
func (n *NodePool) MemoryOversubscriptionEnabled(global *SchedulerConfiguration) bool {

	// Default to the global scheduler config.
	memOversubEnabled := global != nil && global.MemoryOversubscriptionEnabled

	// But overwrite it if the node pool also has it configured.
	poolHasMemOversub := n != nil &&
		n.SchedulerConfiguration != nil &&
		n.SchedulerConfiguration.MemoryOversubscriptionEnabled != nil
	if poolHasMemOversub {
		memOversubEnabled = *n.SchedulerConfiguration.MemoryOversubscriptionEnabled
	}

	return memOversubEnabled
}

// SetHash is used to compute and set the hash of node pool
func (n *NodePool) SetHash() []byte {
	// Initialize a 256bit Blake2 hash (32 bytes)
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	// Write all the user set fields
	_, _ = hash.Write([]byte(n.Name))
	_, _ = hash.Write([]byte(n.Description))
	if n.SchedulerConfiguration != nil {
		_, _ = hash.Write([]byte(n.SchedulerConfiguration.SchedulerAlgorithm))

		memSub := n.SchedulerConfiguration.MemoryOversubscriptionEnabled
		if memSub != nil {
			if *memSub {
				_, _ = hash.Write([]byte("memory_oversubscription_enabled"))
			} else {
				_, _ = hash.Write([]byte("memory_oversubscription_disabled"))
			}
		}
	}

	// sort keys to ensure hash stability when meta is stored later
	var keys []string
	for k := range n.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		_, _ = hash.Write([]byte(k))
		_, _ = hash.Write([]byte(n.Meta[k]))
	}

	// Finalize the hash
	hashVal := hash.Sum(nil)

	// Set and return the hash
	n.Hash = hashVal
	return hashVal
}

// NodePoolSchedulerConfiguration is the scheduler confinguration applied to a
// node pool.
//
// When adding new values that should override global scheduler configuration,
// verify the scheduler handles the node pool configuration as well.
type NodePoolSchedulerConfiguration struct {

	// SchedulerAlgorithm is the scheduling algorithm to use for the pool.
	// If not defined, the global cluster scheduling algorithm is used.
	SchedulerAlgorithm SchedulerAlgorithm `hcl:"scheduler_algorithm"`

	// MemoryOversubscriptionEnabled specifies whether memory oversubscription
	// is enabled. If not defined, the global cluster configuration is used.
	MemoryOversubscriptionEnabled *bool `hcl:"memory_oversubscription_enabled"`
}

// Copy returns a deep copy of the node pool scheduler configuration.
func (n *NodePoolSchedulerConfiguration) Copy() *NodePoolSchedulerConfiguration {
	if n == nil {
		return nil
	}

	nc := new(NodePoolSchedulerConfiguration)
	*nc = *n

	if n.MemoryOversubscriptionEnabled != nil {
		nc.MemoryOversubscriptionEnabled = pointer.Of(*n.MemoryOversubscriptionEnabled)
	}

	return nc
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
