package base

import (
	"strconv"

	"github.com/hashicorp/nomad/plugin-interface/lib/idset"
)

type Topology struct {
	// COMPAT: idset.Set wasn't being serialized correctly but we can't change
	// the encoding of a field once its shipped. Nodes is the wire
	// representation
	nodeIDs *idset.Set[NodeID]
	Nodes   []uint8

	Distances SLIT
	Cores     []Core

	// BusAssociativity maps the specific bus each PCI device is plugged into
	// with its hardware associated numa node
	//
	// e.g. "0000:03:00.0" -> 1
	//
	// Note that the key may not exactly match the Locality.PciBusID from the
	// fingerprint of the device with regard to the domain value.
	//
	//
	// 0000:03:00.0
	// ^    ^  ^  ^
	// |    |  |  |-- function (identifies functionality of device)
	// |    |  |-- device (identifies the device number on the bus)
	// |    |
	// |    |-- bus (identifies which bus segment the device is connected to)
	// |
	// |-- domain (basically always 0, may be 0000 or 00000000)
	BusAssociativity map[string]NodeID

	// explicit overrides from client configuration
	OverrideTotalCompute   MHz
	OverrideWitholdCompute MHz
}

func (t *Topology) Compute() Compute {
	return Compute{}
}

func (st *Topology) SetNodes(nodes *idset.Set[NodeID]) {
	st.nodeIDs = nodes
	if !nodes.Empty() {
		st.Nodes = nodes.Slice()
	} else {
		st.Nodes = []uint8{}
	}
}

// GetNodes returns the set of NUMA Node IDs.
func (st *Topology) GetNodes() *idset.Set[NodeID] {
	if st.nodeIDs.Empty() {
		st.nodeIDs = idset.From[NodeID](st.Nodes)
	}
	return st.nodeIDs
}

type Compute struct {
	TotalCompute MHz `json:"tc"`
	NumCores     int `json:"nc"`
}

type SLIT [][]Cost

type Cost uint8

// A Core represents one logical (vCPU) core on a processor. Basically the slice
// of cores detected should match up with the vCPU description in cloud providers.
type Core struct {
	SocketID   SocketID
	NodeID     NodeID
	ID         CoreID
	Grade      CoreGrade
	Disable    bool // indicates whether Nomad must not use this core
	BaseSpeed  MHz  // cpuinfo_base_freq (primary choice)
	MaxSpeed   MHz  // cpuinfo_max_freq (second choice)
	GuessSpeed MHz  // best effort (fallback)
}

type CoreGrade bool

const (
	Performance CoreGrade = true
	Efficiency  CoreGrade = false
)

type (
	MHz uint64
	KHz uint64
)

func (khz KHz) MHz() MHz {
	return MHz(khz / 1000)
}

func (mhz MHz) KHz() KHz {
	return KHz(mhz * 1000)
}

func (khz KHz) String() string {
	return strconv.FormatUint(uint64(khz.MHz()), 10)
}

type (
	// A NodeID represents a NUMA node. There could be more than
	// one NUMA node per socket.
	//
	// Must be an alias because go-msgpack cannot handle the real type.
	NodeID = uint8

	// A SocketID represents a physical CPU socket.
	SocketID uint8

	// A CoreID represents one logical (vCPU) core.
	CoreID uint16
)
