// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"testing"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/plugins/base/proto"
	"github.com/shoenig/test/must"
)

func Test_nomadTopologyToProto(t *testing.T) {
	top := &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0, 1}),
		Distances: numalib.SLIT{{10}},
		Cores: []numalib.Core{
			{
				SocketID:   0,
				NodeID:     1,
				ID:         2,
				Grade:      numalib.Performance,
				Disable:    false,
				BaseSpeed:  1000,
				MaxSpeed:   5000,
				GuessSpeed: 3000,
			},
		},
		OverrideTotalCompute:   90_000,
		OverrideWitholdCompute: 2000,
	}

	pb := nomadTopologyToProto(top)
	must.Eq(t, &proto.ClientTopology{
		NodeIds: []uint32{0, 1},
		Distances: &proto.ClientTopologySLIT{
			Dimension: 1,
			Values:    []uint32{10},
		},
		Cores: []*proto.ClientTopologyCore{
			{
				SocketId:   0,
				NodeId:     1,
				CoreId:     2,
				CoreGrade:  true,
				Disable:    false,
				BaseSpeed:  1000,
				MaxSpeed:   5000,
				GuessSpeed: 3000,
			},
		},
		OverrideTotalCompute:   90_000,
		OverrideWitholdCompute: 2000,
	}, pb)
}

func Test_nomadTopologyFromProto(t *testing.T) {
	pb := &proto.ClientTopology{
		NodeIds: []uint32{0, 1},
		Distances: &proto.ClientTopologySLIT{
			Dimension: 1,
			Values:    []uint32{10},
		},
		Cores: []*proto.ClientTopologyCore{
			{
				SocketId:   0,
				NodeId:     1,
				CoreId:     2,
				CoreGrade:  true,
				Disable:    false,
				BaseSpeed:  1000,
				MaxSpeed:   5000,
				GuessSpeed: 3000,
			},
		},
		OverrideTotalCompute:   90_000,
		OverrideWitholdCompute: 2000,
	}
	top := nomadTopologyFromProto(pb)
	must.Eq(t, &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0, 1}),
		Distances: numalib.SLIT{{10}},
		Cores: []numalib.Core{
			{
				SocketID:   0,
				NodeID:     1,
				ID:         2,
				Grade:      numalib.Performance,
				Disable:    false,
				BaseSpeed:  1000,
				MaxSpeed:   5000,
				GuessSpeed: 3000,
			},
		},
		OverrideTotalCompute:   90_000,
		OverrideWitholdCompute: 2000,
	}, top)
}

func Test_nomadTopologyDistancesToProto(t *testing.T) {
	slit := numalib.SLIT{
		{10, 20},
		{20, 10},
	}

	pb := nomadTopologyDistancesToProto(slit)
	must.Eq(t, 2, pb.Dimension)
	must.Eq(t, []uint32{10, 20, 20, 10}, pb.Values)
}

func Test_nomadTopologyDistanceFromProto(t *testing.T) {
	pb := &proto.ClientTopologySLIT{
		Dimension: 2,
		Values:    []uint32{10, 20, 20, 10},
	}

	slit := nomadTopologyDistancesFromProto(pb)
	must.Eq(t, numalib.SLIT{
		{10, 20},
		{20, 10},
	}, slit)
}

func Test_nomadTopologyCoresToProto(t *testing.T) {
	cores := []numalib.Core{
		{
			SocketID:   0,
			NodeID:     1,
			ID:         3,
			Grade:      numalib.Efficiency,
			Disable:    false,
			BaseSpeed:  1000,
			MaxSpeed:   3000,
			GuessSpeed: 2200,
		},
		{
			SocketID:   2,
			NodeID:     4,
			ID:         9,
			Grade:      numalib.Performance,
			Disable:    true,
			BaseSpeed:  1500,
			MaxSpeed:   5000,
			GuessSpeed: 3500,
		},
	}

	result := nomadTopologyCoresToProto(cores)
	must.Eq(t, []*proto.ClientTopologyCore{{
		SocketId:   0,
		NodeId:     1,
		CoreId:     3,
		CoreGrade:  bool(numalib.Efficiency),
		Disable:    false,
		BaseSpeed:  1000,
		MaxSpeed:   3000,
		GuessSpeed: 2200,
	}, {
		SocketId:   2,
		NodeId:     4,
		CoreId:     9,
		CoreGrade:  bool(numalib.Performance),
		Disable:    true,
		BaseSpeed:  1500,
		MaxSpeed:   5000,
		GuessSpeed: 3500,
	}}, result)
}

func Test_nomadTopologyCoresFromProto(t *testing.T) {
	pbcores := []*proto.ClientTopologyCore{{
		SocketId:   0,
		NodeId:     1,
		CoreId:     3,
		CoreGrade:  bool(numalib.Efficiency),
		Disable:    false,
		BaseSpeed:  1000,
		MaxSpeed:   3000,
		GuessSpeed: 2200,
	}, {
		SocketId:   2,
		NodeId:     4,
		CoreId:     9,
		CoreGrade:  bool(numalib.Performance),
		Disable:    true,
		BaseSpeed:  1500,
		MaxSpeed:   5000,
		GuessSpeed: 3500,
	}}

	cores := nomadTopologyCoresFromProto(pbcores)
	must.Eq(t, []numalib.Core{{
		SocketID:   0,
		NodeID:     1,
		ID:         3,
		Grade:      numalib.Efficiency,
		Disable:    false,
		BaseSpeed:  1000,
		MaxSpeed:   3000,
		GuessSpeed: 2200,
	}, {
		SocketID:   2,
		NodeID:     4,
		ID:         9,
		Grade:      numalib.Performance,
		Disable:    true,
		BaseSpeed:  1500,
		MaxSpeed:   5000,
		GuessSpeed: 3500,
	}}, cores)
}
