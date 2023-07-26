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
	Latency  uint8
)

func (hz KHz) MHz() MHz {
	return MHz(hz / 1000)
}

func (hz KHz) String() string {
	return strconv.FormatUint(uint64(hz.MHz()), 10)
}

// A Topology provides a bird-eye view of the system NUMA topology.
type Topology struct {
	NodeIDs   *idset.Set[NodeID] `json:"node_ids"`
	Distances Distances          `json:"distances"`
	Cores     []Core             `json:"cores"`

	// explicit overrides from client configuration
	OverrideTotalCompute   MHz `json:"override_total_compute"`
	OverrideWitholdCompute MHz `json:"override_withhold_compute"`
}

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

type Distances [][]Latency

func (d Distances) cost(a, b NodeID) Latency {
	return d[a][b]
}

func (st *Topology) SupportsNUMA() bool {
	switch runtime.GOOS {
	case "linux":
		return true
	default:
		return false
	}
}

func (st *Topology) Nodes() *idset.Set[NodeID] {
	if !st.SupportsNUMA() {
		return nil
	}
	return st.NodeIDs
}

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

func (st *Topology) TotalCompute() MHz {
	var total MHz
	for _, cpu := range st.Cores {
		total += cpu.MHz()
	}
	return total
}

func (st *Topology) UsableCompute() MHz {
	var total MHz
	for _, cpu := range st.Cores {
		if !cpu.Disable {
			total += cpu.MHz()
		}
	}
	return total
}

func (st *Topology) NumCores() int {
	return len(st.Cores)
}

func (st *Topology) NumPCores() int {
	var total int
	for _, cpu := range st.Cores {
		if cpu.Grade == performance {
			total++
		}
	}
	return total
}

func (st *Topology) NumECores() int {
	var total int
	for _, cpu := range st.Cores {
		if cpu.Grade == efficiency {
			total++
		}
	}
	return total
}

func (st *Topology) UsableCores() *idset.Set[CoreID] {
	result := idset.Empty[CoreID]()
	for _, cpu := range st.Cores {
		if !cpu.Disable {
			result.Insert(cpu.ID)
		}
	}
	return result
}

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
