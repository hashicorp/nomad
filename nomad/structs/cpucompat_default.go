// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package structs

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
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
	coreCount := old.TotalCpuCores

	// interpret per-core frequency given total compute and total core count
	frequency := hw.MHz(old.CpuShares / (int64(coreCount)))

	// synthesize a set of cores that abstractly matches the legacy cpu specs
	cores := make([]numalib.Core, 0, coreCount)

	for i := 0; i < int(coreCount); i++ {
		cores = append(cores, numalib.Core{
			ID:         hw.CoreID(i),
			SocketID:   0,                   // no numa support on non-linux
			NodeID:     0,                   // no numa support on non-linux
			Grade:      numalib.Performance, // assume P-cores
			Disable:    false,               // no reservable cores on non-linux
			GuessSpeed: frequency,
		})
	}

	withheld := (frequency * hw.MHz(coreCount)) - hw.MHz(old.CpuShares)

	return &numalib.Topology{
		// legacy: assume one node with id 0
		NodeIDs: idset.From[hw.NodeID]([]hw.NodeID{0}),

		// legacy: with one node the distance matrix is 1-D
		Distances: numalib.SLIT{{10}},

		// legacy: a pseudo representation of each actual core profile
		Cores: cores,

		// legacy: set since we have the value
		OverrideTotalCompute: hw.MHz(old.CpuShares),

		// legacy: set since we can compute the value
		OverrideWitholdCompute: withheld,
	}
}
