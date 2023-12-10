// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package scheduler

import (
	"math/rand"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

type coreSelector struct {
	topology       *numalib.Topology
	availableCores *idset.Set[hw.CoreID]
	shuffle        func([]numalib.Core)
}

// Select returns a set of CoreIDs that satisfy the requested core reservations,
// as well as the amount of CPU bandwidth represented by those specific cores.
//
// NUMA preference is available in ent only.
func (cs *coreSelector) Select(ask *structs.Resources) ([]uint16, hw.MHz) {
	cores := cs.availableCores.Slice()[0:ask.Cores]
	mhz := hw.MHz(0)
	for _, core := range cores {
		mhz += cs.topology.Cores[core].MHz()
	}
	ids := helper.ConvertSlice(cores, func(id hw.CoreID) uint16 { return uint16(id) })
	return ids, mhz
}

// randomize the cores so we can at least try to mitigate PFNR problems
func randomizeCores(cores []numalib.Core) {
	rand.Shuffle(len(cores), func(x, y int) {
		cores[x], cores[y] = cores[y], cores[x]
	})
}
