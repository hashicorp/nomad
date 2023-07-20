// Package numalib provides information regarding the system NUMA, CPU, and
// device topology of the system.
//
// https://docs.kernel.org/6.2/x86/topology.html
package numalib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
)

type (
	nodeID   uint8
	socketID uint8
	coreID   uint16
	Latency  uint8
)

// A Topology provides a bird-eye view of the system NUMA topology.
type Topology struct {
	nodes     *idset.Set[nodeID]
	distances distances
	sockets   []Socket
}

type distances [][]Latency

func (d distances) cost(a, b nodeID) Latency {
	return d[a][b]
}

// Socket represents one physical I.C. that plugs into the motherboard. There
// can be one or more scokets in a system. Also called a package.
type Socket struct {
	id    uint8
	cores []Core
}

type Core struct {
	id      uint16
	threads uint8
	node    uint8
}
