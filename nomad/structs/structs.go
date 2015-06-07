package structs

import (
	"bytes"
	"fmt"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
)

var (
	ErrNoLeader     = fmt.Errorf("No cluster leader")
	ErrNoRegionPath = fmt.Errorf("No path to region")
)

type MessageType uint8

const (
	RegisterRequestType MessageType = iota
)

const (
	// IgnoreUnknownTypeFlag is set along with a MessageType
	// to indicate that the message type can be safely ignored
	// if it is not recognized. This is for future proofing, so
	// that new commands can be added in a way that won't cause
	// old servers to crash when the FSM attempts to process them.
	IgnoreUnknownTypeFlag MessageType = 128
)

// RPCInfo is used to describe common information about query
type RPCInfo interface {
	RequestRegion() string
	IsRead() bool
	AllowStaleRead() bool
}

// QueryOptions is used to specify various flags for read queries
type QueryOptions struct {
	// If set, wait until query exceeds given index. Must be provided
	// with MaxQueryTime.
	MinQueryIndex uint64

	// Provided with MinQueryIndex to wait for change.
	MaxQueryTime time.Duration

	// The target region for this query
	Region string

	// If set, any follower can service the request. Results
	// may be arbitrarily stale.
	AllowStale bool
}

func (q QueryOptions) RequestRegion() string {
	return q.Region
}

// QueryOption only applies to reads, so always true
func (q QueryOptions) IsRead() bool {
	return true
}

func (q QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

type WriteRequest struct {
	Region string
}

func (w WriteRequest) RequestRegion() string {
	// The target region for this request
	return w.Region
}

// WriteRequest only applies to writes, always false
func (w WriteRequest) IsRead() bool {
	return false
}

func (w WriteRequest) AllowStaleRead() bool {
	return false
}

// QueryMeta allows a query response to include potentially
// useful metadata about a query
type QueryMeta struct {
	// This is the index associated with the read
	Index uint64

	// If AllowStale is used, this is time elapsed since
	// last contact between the follower and leader. This
	// can be used to gauge staleness.
	LastContact time.Duration

	// Used to indicate if there is a known leader node
	KnownLeader bool
}

const (
	// CoreCapability is used to convey the client core
	// version. This is a special reserved capability.
	CoreCapability = "core"
)

const (
	StatusInit  = "initializing"
	StatusReady = "ready"
	StatusMaint = "maintenance"
	StatusDown  = "down"
)

// RegisterRequest is used for Client.Register endpoint
// to register a node as being a schedulable entity.
type RegisterRequest struct {
	// Datacenter for this node
	Datacenter string

	// Status of this node
	Status string

	// Scheduling capabilities are used by drivers.
	// e.g. core = 2, docker = 1, java = 2, etc
	Capabilities map[string]int

	// Attributes is an arbitrary set of key/value
	// data that can be used for constraints. Examples
	// include "os=linux", "arch=386", "docker.runtime=1.8.3"
	Attributes map[string]interface{}

	// Resources is the available resources on the client.
	// For example 'cpu=2' 'memory=2048'
	Resouces *Resources

	// Links are used to 'link' this client to external
	// systems. For example 'consul=foo.dc1' 'aws=i-83212'
	// 'ami=ami-123'
	Links map[string]interface{}

	// Meta is used to associate arbitrary metadata with this
	// client. This is opaque to Nomad.
	Meta map[string]string

	WriteRequest
}

// Resources is used to define the resources available
// on a client
type Resources struct {
	CPU              float64
	CPUReserved      float64
	MemoryMB         int
	MemoryMBReserved int
	DiskMB           int
	DiskMBReservered int
	IOPS             int
	IOPSReserved     int
	Networks         []*NetworkResource
	Other            map[string]interface{}
}

// NetworkResource is used to represesent available network
// resources>
type NetworkResource struct {
	Public        bool   // Is this a public address?
	CIDR          string // CIDR block of addresses
	ReservedPorts []int  // Reserved ports
	MBits         int    // Throughput
	MBitsReserved int
}

// msgpackHandle is a shared handle for encoding/decoding of structs
var msgpackHandle = &codec.MsgpackHandle{}

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), msgpackHandle).Decode(out)
}

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(uint8(t))
	err := codec.NewEncoder(&buf, msgpackHandle).Encode(msg)
	return buf.Bytes(), err
}
