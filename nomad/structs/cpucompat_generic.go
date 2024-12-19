// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

func topologyFromLegacyGeneric(old LegacyNodeCpuResources) *numalib.Topology {
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

	t := &numalib.Topology{
		// legacy: with one node the distance matrix is 1-D
		Distances: numalib.SLIT{{10}},

		// legacy: a pseudo representation of each actual core profile
		Cores: cores,

		// legacy: set since we have the value
		OverrideTotalCompute: hw.MHz(old.CpuShares),

		// legacy: set since we can compute the value
		OverrideWitholdCompute: withheld,
	}

	// legacy: assume one node with id 0
	t.SetNodes(idset.From[hw.NodeID]([]hw.NodeID{0}))
	return t
}
