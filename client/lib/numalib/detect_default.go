// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux && !darwin

package numalib

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/shirou/gopsutil/v3/cpu"
)

// PlatformScanners returns the set of SystemScanner for systems without a
// specific implementation.
func PlatformScanners() []SystemScanner {
	return []SystemScanner{
		new(Generic),
	}
}

const (
	nodeID   = NodeID(0)
	socketID = SocketID(0)
	maxSpeed = KHz(0)
)

// Generic implements SystemScanner as a fallback for operating systems without
// a specific implementation.
type Generic struct{}

func (g *Generic) ScanSystem(top *Topology) {
	// hardware may or may not be NUMA, but for now we only
	// detect such topology on linux systems
	top.NodeIDs = idset.Empty[NodeID]()
	top.NodeIDs.Insert(nodeID)

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
		speed := KHz(MHz(info.Mhz) * 1000)
		top.insert(nodeID, socketID, CoreID(i), performance, maxSpeed, speed)
	}
}
