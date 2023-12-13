// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package structs

import (
	"github.com/hashicorp/nomad/client/lib/numalib"
)

// Compatibility will translate the LegacyNodeCpuResources into NodeProcessor
// Resources, or the other way around as needed.
//
// This implementation is specific to non-linux operating systems where
// there are no reservable cores.
func (n *NodeResources) Compatibility() {
	// If resources are not set there is nothing to do.
	if n == nil {
		return
	}

	// Copy values from n.Processors to n.Cpu for compatibility
	//
	// COMPAT: added in Nomad 1.7; can be removed in 1.9+
	if n.Processors.Topology == nil && !n.Cpu.empty() {
		// When we receive a node update from a pre-1.7 client it contains only
		// the LegacyNodeCpuResources field, and so we synthesize a pseudo
		// NodeProcessorResources field
		n.Processors.Topology = topologyFromLegacy(n.Cpu)
	} else if !n.Processors.empty() {
		// When we receive a node update from a 1.7+ client it contains a
		// NodeProcessorResources field, and we populate the LegacyNodeCpuResources
		// field using that information.
		n.Cpu.CpuShares = int64(n.Processors.TotalCompute())
		n.Cpu.TotalCpuCores = uint16(n.Processors.Topology.UsableCores().Size())
	}
}

func topologyFromLegacy(old LegacyNodeCpuResources) *numalib.Topology {
	return topologyFromLegacyGeneric(old)
}
