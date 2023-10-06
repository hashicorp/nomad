// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package numalib provides information regarding the system NUMA, CPU, and
// device topology of the system.
//
// https://docs.kernel.org/6.2/x86/topology.html
package numalib

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// CoreGrade describes whether a specific core is a performance or efficiency
// core type. A performance core generally has a higher clockspeed and can do
// more than an efficiency core.
type CoreGrade bool

const (
	Performance CoreGrade = true
	Efficiency  CoreGrade = false
)

func gradeOf(siblings *idset.Set[hw.CoreID]) CoreGrade {
	switch siblings.Size() {
	case 0, 1:
		return Efficiency
	default:
		return Performance
	}
}

func (g CoreGrade) String() string {
	switch g {
	case Performance:
		return "performance"
	default:
		return "efficiency"
	}
}

type (
	Cost uint8
)

// A Topology provides a bird-eye view of the system NUMA topology.
//
// The JSON encoding is not used yet but my be part of the gRPC plumbing
// in the future.
type Topology struct {
	NodeIDs   *idset.Set[hw.NodeID]
	Distances SLIT
	Cores     []Core

	// explicit overrides from client configuration
	OverrideTotalCompute   hw.MHz
	OverrideWitholdCompute hw.MHz
}

// A Core represents one logical (vCPU) core on a processor. Basically the slice
// of cores detected should match up with the vCPU description in cloud providers.
type Core struct {
	SocketID   hw.SocketID
	NodeID     hw.NodeID
	ID         hw.CoreID
	Grade      CoreGrade
	Disable    bool   // indicates whether Nomad must not use this core
	BaseSpeed  hw.MHz // cpuinfo_base_freq (primary choice)
	MaxSpeed   hw.MHz // cpuinfo_max_freq (second choice)
	GuessSpeed hw.MHz // best effort (fallback)
}

func (c Core) String() string {
	return fmt.Sprintf(
		"(%d %d %d %s %d %d)",
		c.NodeID, c.SocketID, c.ID, c.Grade, c.MaxSpeed, c.BaseSpeed,
	)
}

func (c Core) MHz() hw.MHz {
	switch {
	case c.BaseSpeed > 0:
		return c.BaseSpeed
	case c.MaxSpeed > 0:
		return c.MaxSpeed
	}
	return c.GuessSpeed
}

// SLIT (system locality information table) describes the relative cost for
// accessing memory across each combination of NUMA boundary.
type SLIT [][]Cost

func (d SLIT) cost(a, b hw.NodeID) Cost {
	return d[a][b]
}

// SupportsNUMA returns whether Nomad supports NUMA detection on the client's
// operating system. Currently only supported on Linux.
func (st *Topology) SupportsNUMA() bool {
	switch runtime.GOOS {
	case "linux":
		return true
	default:
		return false
	}
}

// Nodes returns the set of NUMA Node IDs.
func (st *Topology) Nodes() *idset.Set[hw.NodeID] {
	if !st.SupportsNUMA() {
		return nil
	}
	return st.NodeIDs
}

// NodeCores returns the set of Core IDs for the given NUMA Node ID.
func (st *Topology) NodeCores(node hw.NodeID) *idset.Set[hw.CoreID] {
	result := idset.Empty[hw.CoreID]()
	for _, cpu := range st.Cores {
		if cpu.NodeID == node {
			result.Insert(cpu.ID)
		}
	}
	return result
}

func (st *Topology) insert(node hw.NodeID, socket hw.SocketID, core hw.CoreID, grade CoreGrade, max, base hw.KHz) {
	st.Cores[core] = Core{
		NodeID:    node,
		SocketID:  socket,
		ID:        core,
		Grade:     grade,
		MaxSpeed:  max.MHz(),
		BaseSpeed: base.MHz(),
	}
}

func (st *Topology) String() string {
	var sb strings.Builder
	for _, cpu := range st.Cores {
		sb.WriteString(cpu.String())
	}
	return sb.String()
}

// TotalCompute returns the amount of compute in MHz the detected hardware is
// ultimately capable of delivering. The UsableCompute will be equal to or
// less than this value.
//
// If the client configuration includes an override for total compute, that
// value is used instead even if it violates the above invariant.
func (st *Topology) TotalCompute() hw.MHz {
	if st.OverrideTotalCompute > 0 {
		// TODO(shoenig) Starting in Nomad 1.7 we should warn about setting
		// cpu_total_compute override, and suggeset users who think they still
		// need this to file a bug so we can understand what is not detectable.
		return st.OverrideTotalCompute
	}

	var total hw.MHz
	for _, cpu := range st.Cores {
		total += cpu.MHz()
	}
	return total
}

// UsableCompute returns the amount of compute in MHz the Nomad client is able
// to make use of for running tasks. This value will be less than or equal to
// the TotalCompute of the system. Nomad must subtract off any reserved compute
// (reserved.cpu or reserved.cores) from the total hardware compute.
func (st *Topology) UsableCompute() hw.MHz {
	if st.OverrideTotalCompute > 0 {
		// TODO(shoenig) Starting in Nomad 1.7 we should warn about setting
		// cpu_total_compute override, and suggeset users who think they still
		// need this to file a bug so we can understand what is not detectable.
		return st.OverrideTotalCompute
	}

	var total hw.MHz
	for _, cpu := range st.Cores {
		// only use cores allowable by config
		if !cpu.Disable {
			total += cpu.MHz()
		}
	}

	// only use compute allowable by config
	return total - st.OverrideWitholdCompute
}

// NumCores returns the number of logical cores detected. This includes both
// power and efficiency cores.
func (st *Topology) NumCores() int {
	return len(st.Cores)
}

// NumPCores returns the number of logical performance cores detected.
func (st *Topology) NumPCores() int {
	var total int
	for _, cpu := range st.Cores {
		if cpu.Grade == Performance {
			total++
		}
	}
	return total
}

// NumECores returns the number of logical efficiency cores detected.
func (st *Topology) NumECores() int {
	var total int
	for _, cpu := range st.Cores {
		if cpu.Grade == Efficiency {
			total++
		}
	}
	return total
}

// UsableCores returns the number of logical cores usable by the Nomad client
// for running tasks. Nomad must subtract off any reserved cores (reserved.cores)
// and/or must mask the cpuset to the one set in config (config.reservable_cores).
func (st *Topology) UsableCores() *idset.Set[hw.CoreID] {
	result := idset.Empty[hw.CoreID]()
	for _, cpu := range st.Cores {
		if !cpu.Disable {
			result.Insert(cpu.ID)
		}
	}
	return result
}

// CoreSpeeds returns the frequency in MHz of the performance and efficiency
// core types. If the CPU does not have effiency cores that value will be zero.
func (st *Topology) CoreSpeeds() (hw.MHz, hw.MHz) {
	var pCore, eCore hw.MHz
	for _, cpu := range st.Cores {
		switch cpu.Grade {
		case Performance:
			pCore = cpu.MHz()
		case Efficiency:
			eCore = cpu.MHz()
		}
	}
	return pCore, eCore
}

func (st *Topology) Compute() cpustats.Compute {
	return cpustats.Compute{
		TotalCompute: st.TotalCompute(),
		NumCores:     st.NumCores(),
	}
}

func (st *Topology) Equal(o *Topology) bool {
	if st == nil || o == nil {
		return st == o
	}
	// simply iterates each core; the topology never changes for a node once
	// it has been created at agent startup
	return st.TotalCompute() == o.TotalCompute()
}
