// Package numalib provides information regarding the system NUMA, CPU, and
// device topology of the system.
//
// https://docs.kernel.org/6.2/x86/topology.html
package numalib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
)

type grade bool

const (
	performance grade = true
	efficiency  grade = false
)

func gradeOf(siblings *idset.Set[coreID]) grade {
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
	nodeID   uint8
	socketID uint8
	coreID   uint16
	Hz       uint64
	MHz      uint64
	GHz      float64
	Latency  uint8
)

func (hz Hz) MHz() MHz {
	return MHz(hz / 1_000_000)
}

func (hz Hz) String() string {
	return strconv.FormatUint(uint64(hz.MHz()), 10)
}

// A Topology provides a bird-eye view of the system NUMA topology.
type Topology struct {
	nodes     *idset.Set[nodeID]
	distances distances
	cpus      []Core
}

type Core struct {
	node   nodeID
	socket socketID
	id     coreID
	grade  grade
	base   Hz // cpuinfo_base_freq (primary choice)
	max    Hz // cpuinfo_max_freq (second choice)
	guess  Hz // best effort (fallback)
}

func (c Core) String() string {
	return fmt.Sprintf(
		"(%d %d %d %s %d %d)",
		c.node, c.socket, c.id, c.grade, c.max, c.base,
	)
}

func (c Core) Hz() Hz {
	switch {
	case c.base > 0:
		return c.base
	case c.max > 0:
		return c.max
	}
	return c.guess
}

type distances [][]Latency

func (d distances) cost(a, b nodeID) Latency {
	return d[a][b]
}

func (st *Topology) insert(node nodeID, socket socketID, core coreID, grade grade, max, base Hz) {
	st.cpus[core] = Core{
		node:   node,
		socket: socket,
		id:     core,
		grade:  grade,
		max:    max,
		base:   base,
	}
}

func (st *Topology) String() string {
	var sb strings.Builder
	for _, cpu := range st.cpus {
		sb.WriteString(cpu.String())
	}
	return sb.String()
}

func (st *Topology) TotalCompute() Hz {
	var total Hz
	for _, cpu := range st.cpus {
		total += cpu.Hz()
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

func (st *Topology) CoreSpeeds() (Hz, Hz) {
	var pCore, eCore Hz
	for _, cpu := range st.cpus {
		switch cpu.grade {
		case performance:
			pCore = cpu.Hz()
		case efficiency:
			eCore = cpu.Hz()
		}
	}
	return pCore, eCore
}
