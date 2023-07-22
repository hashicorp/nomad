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

type grade bool

const (
	performance grade = true
	efficiency  grade = false
)

func gradeOf(siblings *idset.Set[CoreID]) grade {
	switch siblings.Size() {
	case 0, 1:
		return efficiency
	default:
		return performance
	}
}

func (g grade) String() string {
	switch g {
	case performance:
		return "performance"
	default:
		return "efficiency"
	}
}

type (
	NodeID   uint8
	socketID uint8
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
	nodes     *idset.Set[NodeID]
	distances distances
	cpus      []Core

	// explicit overrides from client configuration
	overrideTotalCompute   MHz
	overrideWitholdCompute MHz
}

type Core struct {
	node    NodeID
	socket  socketID
	id      CoreID
	grade   grade
	disable bool // indicates whether Nomad must not use this core
	base    MHz  // cpuinfo_base_freq (primary choice)
	max     MHz  // cpuinfo_max_freq (second choice)
	guess   MHz  // best effort (fallback)
}

func (c Core) String() string {
	return fmt.Sprintf(
		"(%d %d %d %s %d %d)",
		c.node, c.socket, c.id, c.grade, c.max, c.base,
	)
}

func (c Core) MHz() MHz {
	switch {
	case c.base > 0:
		return c.base
	case c.max > 0:
		return c.max
	}
	return c.guess
}

type distances [][]Latency

func (d distances) cost(a, b NodeID) Latency {
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
	return st.nodes
}

func (st *Topology) NodeCores(node NodeID) *idset.Set[CoreID] {
	result := idset.Empty[CoreID]()
	for _, cpu := range st.cpus {
		if cpu.node == node {
			result.Insert(cpu.id)
		}
	}
	return result
}

func (st *Topology) insert(node NodeID, socket socketID, core CoreID, grade grade, max, base KHz) {
	st.cpus[core] = Core{
		node:   node,
		socket: socket,
		id:     core,
		grade:  grade,
		max:    max.MHz(),
		base:   base.MHz(),
	}
}

func (st *Topology) String() string {
	var sb strings.Builder
	for _, cpu := range st.cpus {
		sb.WriteString(cpu.String())
	}
	return sb.String()
}

func (st *Topology) TotalCompute() MHz {
	var total MHz
	for _, cpu := range st.cpus {
		total += cpu.MHz()
	}
	return total
}

func (st *Topology) UsableCompute() MHz {
	var total MHz
	for _, cpu := range st.cpus {
		if !cpu.disable {
			total += cpu.MHz()
		}
	}
	return total
}

func (st *Topology) NumCores() int {
	return len(st.cpus)
}

func (st *Topology) NumPCores() int {
	var total int
	for _, cpu := range st.cpus {
		if cpu.grade == performance {
			total++
		}
	}
	return total
}

func (st *Topology) NumECores() int {
	var total int
	for _, cpu := range st.cpus {
		if cpu.grade == efficiency {
			total++
		}
	}
	return total
}

func (st *Topology) UsableCores() *idset.Set[CoreID] {
	result := idset.Empty[CoreID]()
	for _, cpu := range st.cpus {
		if !cpu.disable {
			result.Insert(cpu.id)
		}
	}
	return result
}

func (st *Topology) CoreSpeeds() (MHz, MHz) {
	var pCore, eCore MHz
	for _, cpu := range st.cpus {
		switch cpu.grade {
		case performance:
			pCore = cpu.MHz()
		case efficiency:
			eCore = cpu.MHz()
		}
	}
	return pCore, eCore
}
