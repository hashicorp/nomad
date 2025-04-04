// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package scheduler

import (
	"cmp"
	"math/rand"
	"slices"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/nomad/structs"
)

type coreSelector struct {
	topology         *numalib.Topology
	availableCores   *idset.Set[hw.CoreID]
	shuffle          func([]numalib.Core)
	deviceMemoryNode int
}

// Select returns a set of CoreIDs that satisfy the requested core reservations,
// as well as the amount of CPU bandwidth represented by those specific cores.
//
// NUMA preference is available in ent only.
func (cs *coreSelector) Select(ask *structs.Resources) ([]uint16, hw.MHz) {
	cores := cs.availableCores.Slice()[0:ask.Cores]
	mhz := hw.MHz(0)
	ids := make([]uint16, 0, ask.Cores)
	sortedTopologyCores := make([]numalib.Core, len(cs.topology.Cores))
	copy(sortedTopologyCores, cs.topology.Cores)
	slices.SortFunc(sortedTopologyCores, func(a, b numalib.Core) int { return cmp.Compare(a.ID, b.ID) })
	for _, core := range cores {
		if i, found := slices.BinarySearchFunc(sortedTopologyCores, core, func(c numalib.Core, id hw.CoreID) int { return cmp.Compare(c.ID, id) }); found {
			mhz += cs.topology.Cores[i].MHz()
			ids = append(ids, uint16(cs.topology.Cores[i].ID))
		}
	}
	return ids, mhz
}

// randomize the cores so we can at least try to mitigate PFNR problems
func randomizeCores(cores []numalib.Core) {
	rand.Shuffle(len(cores), func(x, y int) {
		cores[x], cores[y] = cores[y], cores[x]
	})
}

// candidateMemoryNodes return -1 on CE, indicating any memory node is acceptable
//
// (NUMA aware scheduling is an enterprise feature)
func (cs *coreSelector) candidateMemoryNodes(ask *structs.Resources) []int {
	return []int{-1}
}
