// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tests

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/nomad/structs"
)

// CpuResources creates both the legacy and modern structs concerning cpu
// metrics used for resource accounting
//
// only creates a trivial single node, single core system for the sake of
// compatibility with existing tests
func CpuResources(shares int) (structs.LegacyNodeCpuResources, structs.NodeProcessorResources) {
	n := &structs.NodeResources{
		Processors: structs.NodeProcessorResources{
			Topology: &numalib.Topology{
				Distances: numalib.SLIT{[]numalib.Cost{10}},
				Cores: []numalib.Core{{
					SocketID:  0,
					NodeID:    0,
					ID:        0,
					Grade:     numalib.Performance,
					Disable:   false,
					BaseSpeed: hw.MHz(shares),
				}},
			},
		},
	}
	n.Processors.Topology.SetNodes(idset.From[hw.NodeID]([]hw.NodeID{0}))

	// polyfill the legacy struct
	n.Compatibility()

	return n.Cpu, n.Processors
}

func CpuResourcesFrom(top *numalib.Topology) (structs.LegacyNodeCpuResources, structs.NodeProcessorResources) {
	n := &structs.NodeResources{
		Processors: structs.NodeProcessorResources{
			Topology: top,
		},
	}

	// polyfill the legacy struct
	n.Compatibility()

	return n.Cpu, n.Processors
}
