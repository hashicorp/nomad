// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shirou/gopsutil/v3/cpu"
)

const (
	genericNodeID   = hw.NodeID(0)
	genericSocketID = hw.SocketID(0)
	genericMaxSpeed = hw.KHz(0)
)

func scanGeneric(top *Topology) {
	// hardware may or may not be NUMA, but for now we only
	// detect such topology on linux systems
	top.nodeIDs = idset.Empty[hw.NodeID]()
	top.nodeIDs.Insert(genericNodeID)
	top.Nodes = top.nodeIDs.Slice()

	// cores
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return
	}
	top.Cores = make([]Core, count)

	infos, err := cpu.InfoWithContext(ctx)
	if err != nil || len(infos) == 0 {
		return
	}

	for i := 0; i < count; i++ {
		info := infos[0]
		speed := hw.KHz(hw.MHz(info.Mhz) * 1000)
		top.insert(genericNodeID, genericSocketID, hw.CoreID(i), Performance, genericMaxSpeed, speed)
	}
}
