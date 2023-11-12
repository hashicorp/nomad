// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

func TestNUMA_topologyFromLegacy_plain(t *testing.T) {
	ci.Parallel(t)

	old := LegacyNodeCpuResources{
		CpuShares:     12800,
		TotalCpuCores: 4,
		ReservableCpuCores: []uint16{
			0, 1, 2, 3,
		},
	}

	result := topologyFromLegacy(old)

	exp := &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0}),
		Distances: numalib.SLIT{{10}},
		Cores: []numalib.Core{
			makeLegacyCore(0),
			makeLegacyCore(1),
			makeLegacyCore(2),
			makeLegacyCore(3),
		},
		OverrideTotalCompute:   12800,
		OverrideWitholdCompute: 0,
	}

	// only compares total compute
	must.Equal(t, exp, result)

	// check underlying fields
	must.Eq(t, exp.NodeIDs, result.NodeIDs)
	must.Eq(t, exp.Distances, result.Distances)
	must.Eq(t, exp.Cores, result.Cores)
	must.Eq(t, exp.OverrideTotalCompute, result.OverrideTotalCompute)
	must.Eq(t, exp.OverrideWitholdCompute, result.OverrideWitholdCompute)
}

func TestNUMA_topologyFromLegacy_reservations(t *testing.T) {
	ci.Parallel(t)

	old := LegacyNodeCpuResources{
		CpuShares:     9600,
		TotalCpuCores: 4,
		ReservableCpuCores: []uint16{
			1, 2, 3, // core 0 excluded
		},
	}

	result := topologyFromLegacy(old)

	exp := &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0}),
		Distances: numalib.SLIT{{10}},
		Cores: []numalib.Core{
			makeLegacyCore(1),
			makeLegacyCore(2),
			makeLegacyCore(3),
		},
		OverrideTotalCompute:   9600,
		OverrideWitholdCompute: 3200, // core 0 excluded
	}

	// only compares total compute
	must.Equal(t, exp, result)

	// check underlying fields
	must.Eq(t, exp.NodeIDs, result.NodeIDs)
	must.Eq(t, exp.Distances, result.Distances)
	must.Eq(t, exp.Cores, result.Cores)
	must.Eq(t, exp.OverrideTotalCompute, result.OverrideTotalCompute)
	must.Eq(t, exp.OverrideWitholdCompute, result.OverrideWitholdCompute)
}
