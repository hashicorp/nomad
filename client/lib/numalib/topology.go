// Package numalib provides information regarding the system NUMA, CPU, and
// device topology of the system.
//
// https://docs.kernel.org/6.2/x86/topology.html
package numalib

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/client/lib/idset"
)

type (
	nodeID   uint8
	socketID uint8
	coreID   uint16
	hz       uint64
	Latency  uint8
)

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
	max    hz
	base   hz
}

func (c Core) String() string {
	return fmt.Sprintf(
		"(%d %d %d %d %d)",
		c.node, c.socket, c.id, c.max, c.base,
	)
}

type distances [][]Latency

func (d distances) cost(a, b nodeID) Latency {
	return d[a][b]
}

func (st *Topology) insert(node nodeID, socket socketID, core coreID, max, base hz) {
	st.cpus[core] = Core{
		node:   node,
		socket: socket,
		id:     core,
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
