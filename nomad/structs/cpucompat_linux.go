// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package structs

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper"
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
		cores := n.Processors.Topology.UsableCores().Slice()
		n.Cpu.ReservableCpuCores = helper.ConvertSlice(cores, func(coreID hw.CoreID) uint16 {
			return uint16(coreID)
		})
	}
}

func topologyFromLegacy(old LegacyNodeCpuResources) *numalib.Topology {
	// interpret per-core frequency given total compute and total core count
	frequency := hw.MHz(old.CpuShares / (int64(len(old.ReservableCpuCores))))

	cores := helper.ConvertSlice(
		old.ReservableCpuCores,
		func(id uint16) numalib.Core {
			return numalib.Core{
				ID:         hw.CoreID(id),
				SocketID:   0, // legacy: assume single socket with id 0
				NodeID:     0, // legacy: assume single numa node with id 0
				Grade:      numalib.Performance,
				Disable:    false, // only usable cores in the source
				GuessSpeed: frequency,
			}
		},
	)

	withheld := (frequency * hw.MHz(old.TotalCpuCores)) - hw.MHz(old.CpuShares)

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
