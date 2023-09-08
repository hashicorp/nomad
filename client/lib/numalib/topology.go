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
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
)

// CoreGrade describes whether a specific core is a performance or efficiency
// core type. A performance core generally has a higher clockspeed and can do
// more than an efficiency core.
type CoreGrade bool

const (
	performance CoreGrade = true
	efficiency  CoreGrade = false
)

func gradeOf(siblings *idset.Set[CoreID]) CoreGrade {
	switch siblings.Size() {
	case 0, 1:
		return efficiency
	default:
		return performance
	}
}

func (g CoreGrade) String() string {
	switch g {
	case performance:
		return "performance"
	default:
		return "efficiency"
	}
}

type (
	NodeID   uint8
	SocketID uint8
	CoreID   uint16
	KHz      uint64
	MHz      uint64
	GHz      float64
	Cost     uint8
)

func (khz KHz) MHz() MHz {
	return MHz(khz / 1000)
}

func (khz KHz) String() string {
	return strconv.FormatUint(uint64(khz.MHz()), 10)
}

// A Topology provides a bird-eye view of the system NUMA topology.
//
// The JSON encoding is not used yet but my be part of the gRPC plumbing
// in the future.
type Topology struct {
	NodeIDs   *idset.Set[NodeID] `json:"node_ids"`
	Distances SLIT               `json:"distances"`
	Cores     []Core             `json:"cores"`

	// explicit overrides from client configuration
	OverrideTotalCompute   MHz `json:"override_total_compute"`
	OverrideWitholdCompute MHz `json:"override_withhold_compute"`
}

// A Core represents one logical (vCPU) core on a processor. Basically the slice
// of cores detected should match up with the vCPU description in cloud providers.
type Core struct {
	NodeID     NodeID    `json:"node_id"`
	SocketID   SocketID  `json:"socket_id"`
	ID         CoreID    `json:"id"`
	Grade      CoreGrade `json:"grade"`
	Disable    bool      `json:"disable"`     // indicates whether Nomad must not use this core
	BaseSpeed  MHz       `json:"base_speed"`  // cpuinfo_base_freq (primary choice)
	MaxSpeed   MHz       `json:"max_speed"`   // cpuinfo_max_freq (second choice)
	GuessSpeed MHz       `json:"guess_speed"` // best effort (fallback)
}

func (c Core) String() string {
	return fmt.Sprintf(
		"(%d %d %d %s %d %d)",
		c.NodeID, c.SocketID, c.ID, c.Grade, c.MaxSpeed, c.BaseSpeed,
	)
}

func (c Core) MHz() MHz {
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

func (d SLIT) cost(a, b NodeID) Cost {
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
func (st *Topology) Nodes() *idset.Set[NodeID] {
	if !st.SupportsNUMA() {
		return nil
	}
	return st.NodeIDs
}

// NodeCores returns the set of Core IDs for the given NUMA Node ID.
func (st *Topology) NodeCores(node NodeID) *idset.Set[CoreID] {
	result := idset.Empty[CoreID]()
	for _, cpu := range st.Cores {
		if cpu.NodeID == node {
			result.Insert(cpu.ID)
		}
	}
	return result
}

func (st *Topology) insert(node NodeID, socket SocketID, core CoreID, grade CoreGrade, max, base KHz) {
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
func (st *Topology) TotalCompute() MHz {
	if st.OverrideTotalCompute > 0 {
		return st.OverrideTotalCompute
	}

	var total MHz
	for _, cpu := range st.Cores {
		total += cpu.MHz()
	}
	return total
}

// UsableCompute returns the amount of compute in MHz the Nomad client is able
// to make use of for running tasks. This value will be less than or equal to
// the TotalCompute of the system. Nomad must subtract off any reserved compute
// (reserved.cpu or reserved.cores) from the total hardware compute.
func (st *Topology) UsableCompute() MHz {
	var total MHz
	for _, cpu := range st.Cores {
		if !cpu.Disable {
			total += cpu.MHz()
		}
	}
	return total
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
		if cpu.Grade == performance {
			total++
		}
	}
	return total
}

// NumECores returns the number of logical efficiency cores detected.
func (st *Topology) NumECores() int {
	var total int
	for _, cpu := range st.Cores {
		if cpu.Grade == efficiency {
			total++
		}
	}
	return total
}

// UsableCores returns the number of logical cores usable by the Nomad client
// for running tasks. Nomad must subtract off any reserved cores (reserved.cores)
// and/or must mask the cpuset to the one set in config (config.reservable_cores).
func (st *Topology) UsableCores() *idset.Set[CoreID] {
	result := idset.Empty[CoreID]()
	for _, cpu := range st.Cores {
		if !cpu.Disable {
			result.Insert(cpu.ID)
		}
	}
	return result
}

// CoreSpeeds returns the frequency in MHz of the performance and efficiency
// core types. If the CPU does not have effiency cores that value will be zero.
func (st *Topology) CoreSpeeds() (MHz, MHz) {
	var pCore, eCore MHz
	for _, cpu := range st.Cores {
		switch cpu.Grade {
		case performance:
			pCore = cpu.MHz()
		case efficiency:
			eCore = cpu.MHz()
		}
	}
	return pCore, eCore
}
