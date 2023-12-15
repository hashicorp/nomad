// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"

	"github.com/hashicorp/nomad/client/lib/numalib"
)

const (
	// NoneNUMA indicates no NUMA aware scheduling is requested for the task
	NoneNUMA = "none"

	// PreferNUMA indicates nodes with NUMA ideal cores should be used if available
	PreferNUMA = "prefer"

	// RequireNUMA indicates a task must be placed on a node with available NUMA ideal cores
	RequireNUMA = "require"
)

type NUMA struct {
	// Affinity is the numa affinity scheduling behavior.
	// One of "none", "prefer", "require".
	Affinity string
}

func (n *NUMA) Equal(o *NUMA) bool {
	if n == nil || o == nil {
		return n == o
	}
	return n.Affinity == o.Affinity
}

func (n *NUMA) Copy() *NUMA {
	if n == nil {
		return nil
	}
	return &NUMA{
		Affinity: n.Affinity,
	}
}

func (n *NUMA) Validate() error {
	if n == nil {
		return nil
	}
	switch n.Affinity {
	case NoneNUMA, PreferNUMA, RequireNUMA:
		return nil
	default:
		return errors.New("numa affinity must be one of none, prefer, or require")
	}
}

// Requested returns true if the NUMA.Affinity is set to one of "prefer" or
// "require" and will require such CPU cores for scheduling.
func (n *NUMA) Requested() bool {
	if n == nil || n.Affinity == NoneNUMA {
		return false
	}
	return true
}

// LegacyNodeCpuResources is the pre-1.7 CPU resources struct. It remains here
// for compatibility and can be removed in Nomad 1.9+.
//
// Deprecated; use NodeProcessorResources instead.
type LegacyNodeCpuResources struct {
	// Deprecated; do not use this value except for compatibility.
	CpuShares int64

	// Deprecated; do not use this value except for compatibility.
	TotalCpuCores uint16

	// Deprecated; do not use this value except for compatibility.
	ReservableCpuCores []uint16
}

// partial struct serialization / copy / merge sadness means this struct can
// exist with no data, which is a condition we must detect during the upgrade path
func (r LegacyNodeCpuResources) empty() bool {
	return r.CpuShares == 0 || r.TotalCpuCores == 0
}

// NomadProcessorResources captures the CPU hardware resources of the Nomad node.
//
// In Nomad enterprise this structure is used to map tasks to NUMA nodes.
type NodeProcessorResources struct {
	// Topology is here to serve as a reference
	Topology *numalib.Topology // do not modify
}

// partial struct serialization / copy / merge sadness means this struct can
// exist with no data, which is a condition we must detect during the upgrade path
func (r NodeProcessorResources) empty() bool {
	return r.Topology == nil || len(r.Topology.Cores) == 0
}

func NewNodeProcessorResources(top *numalib.Topology) NodeProcessorResources {
	return NodeProcessorResources{
		Topology: top,
	}
}

func (r *NodeProcessorResources) String() string {
	if r == nil || r.Topology == nil {
		return "(nil)"
	}
	return fmt.Sprintf("(%d,%d)", r.Topology.NumECores(), r.Topology.NumPCores())
}

func (r *NodeProcessorResources) Copy() NodeProcessorResources {
	return NodeProcessorResources{
		Topology: r.Topology,
	}
}

func (r *NodeProcessorResources) Merge(o *NodeProcessorResources) {
	if o == nil || o.Topology == nil {
		return
	}
	r.Topology = o.Topology
}

func (r *NodeProcessorResources) Equal(o *NodeProcessorResources) bool {
	if r == nil || o == nil {
		return r == o
	}
	return r.Topology.Equal(o.Topology)
}

func (r *NodeProcessorResources) TotalCompute() int {
	if r == nil || r.Topology == nil {
		return 0
	}
	return int(r.Topology.TotalCompute())
}
