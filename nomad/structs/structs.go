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
	NodeRegisterRequestType MessageType = iota
	NodeDeregisterRequestType
	NodeUpdateStatusRequestType
	JobRegisterRequestType
	JobDeregisterRequestType
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
	// The target region for this query
	Region string

	// If set, wait until query exceeds given index. Must be provided
	// with MaxQueryTime.
	MinQueryIndex uint64

	// Provided with MinQueryIndex to wait for change.
	MaxQueryTime time.Duration

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
	// The target region for this write
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

// WriteMeta allows a write response to includ e potentially
// useful metadata about the write
type WriteMeta struct {
	// This is the index associated with the write
	Index uint64
}

// NodeRegisterRequest is used for Client.Register endpoint
// to register a node as being a schedulable entity.
type NodeRegisterRequest struct {
	Node *Node
	WriteRequest
}

// NodeDeregisterRequest is used for Client.Deregister endpoint
// to deregister a node as being a schedulable entity.
type NodeDeregisterRequest struct {
	NodeID string
	WriteRequest
}

// UpdateStatusRequest is used for Client.UpdateStatus endpoint
// to update the status of a node.
type NodeUpdateStatusRequest struct {
	NodeID string
	Status string
	WriteRequest
}

// NodeSpecificRequest is used when we just need to specify a target node
type NodeSpecificRequest struct {
	NodeID string
	WriteRequest
}

// JobRegisterRequest is used for Job.Register endpoint
// to register a job as being a schedulable entity.
type JobRegisterRequest struct {
	Job *Job
	WriteRequest
}

// JobDeregisterRequest is used for Job.Deregister endpoint
// to deregister a job as being a schedulable entity.
type JobDeregisterRequest struct {
	JobID string
	WriteRequest
}

// JobSpecificRequest is used when we just need to specify a target job
type JobSpecificRequest struct {
	JobID string
	WriteRequest
}

// GenericResponse is used to respond to a request where no
// specific response information is needed.
type GenericResponse struct {
	WriteMeta
}

// SingleNodeResponse is used to return a single node
type SingleNodeResponse struct {
	Node *Node
	QueryMeta
}

// SingleJobResponse is used to return a single job
type SingleJobResponse struct {
	Job *Job
	QueryMeta
}

const (
	NodeStatusInit  = "initializing"
	NodeStatusReady = "ready"
	NodeStatusMaint = "maintenance"
	NodeStatusDown  = "down"
)

// Node is a representation of a schedulable client node
type Node struct {
	// ID is a unique identifier for the node. It can be constructed
	// by doing a concatenation of the Name and Datacenter as a simple
	// approach. Alternatively a UUID may be used.
	ID string

	// Datacenter for this node
	Datacenter string

	// Node name
	Name string

	// Attributes is an arbitrary set of key/value
	// data that can be used for constraints. Examples
	// include "os=linux", "arch=386", "driver.docker=1",
	// "docker.runtime=1.8.3"
	Attributes map[string]string

	// Resources is the available resources on the client.
	// For example 'cpu=2' 'memory=2048'
	Resources *Resources

	// Reserved is the set of resources that are reserved,
	// and should be subtracted from the total resources for
	// the purposes of scheduling. This may be provide certain
	// high-watermark tolerances or because of external schedulers
	// consuming resources.
	Reserved *Resources

	// Allocated is the set of resources that have been allocated
	// as part of scheduling. They should also be excluded for the
	// purposes of additional scheduling allocations.
	Allocated *Resources

	// Links are used to 'link' this client to external
	// systems. For example 'consul=foo.dc1' 'aws=i-83212'
	// 'ami=ami-123'
	Links map[string]string

	// Meta is used to associate arbitrary metadata with this
	// client. This is opaque to Nomad.
	Meta map[string]string

	// NodeClass is an opaque identifier used to group nodes
	// together for the purpose of determining scheduling pressure.
	NodeClass string

	// Status of this node
	Status string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// Resources is used to define the resources available
// on a client
type Resources struct {
	CPU      float64
	MemoryMB int
	DiskMB   int
	IOPS     int
	Networks []*NetworkResource
	Other    map[string]interface{}
}

// NetworkResource is used to represesent available network
// resources
type NetworkResource struct {
	Public        bool   // Is this a public address?
	CIDR          string // CIDR block of addresses
	ReservedPorts []int  // Reserved ports
	MBits         int    // Throughput
}

const (
	JobTypeService = "service"
	JobTypeBatch   = "batch"
)

const (
	JobStatusPending  = "pending"  // Pending means the job is waiting on scheduling
	JobStatusRunning  = "running"  // Running means the entire job is running
	JobStatusComplete = "complete" // Complete means there was a clean termination
	JobStatusDead     = "dead"     // Dead means there was abnormal termination
)

// Job is the scope of a scheduling request to Nomad. It is the largest
// scoped object, and is a named collection of task groups. Each task group
// is further composed of tasks. A task group (TG) is the unit of scheduling
// however.
type Job struct {
	// ID is a unique identifier for the job. It can be the same as
	// the job name, or alternatively a UUID may be used.
	ID string

	// Name is the logical name of the job used to refer to it. This is unique
	// per region, but not unique globally.
	Name string

	// Type is used to control various behaviors about the job. Most jobs
	// are service jobs, meaning they are expected to be long lived.
	// Some jobs are batch oriented meaning they run and then terminate.
	// This can be extended in the future to support custom schedulers.
	Type string

	// Priority is used to control scheduling importance and if this job
	// can preempt other jobs.
	Priority int

	// AllAtOnce is used to control if incremental scheduling of task groups
	// is allowed or if we must do a gang scheduling of the entire job. This
	// can slow down larger jobs if resources are not available.
	AllAtOnce bool

	// Constraints can be specified at a job level and apply to
	// all the task groups and tasks.
	Constraints []*Constraint

	// TaskGroups are the collections of task groups that this job needs
	// to run. Each task group is an atomic unit of scheduling and placement.
	TaskGroups []*TaskGroup

	// Meta is used to associate arbitrary metadata with this
	// job. This is opaque to Nomad.
	Meta map[string]string

	// Job status
	Status string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// TaskGroup is an atomic unit of placement. Each task group belongs to
// a job and may contain any number of tasks. A task group support running
// in many replicas using the same configuration..
type TaskGroup struct {
	// Name of the task group
	Name string

	// Count is the number of replicas of this task group that should
	// be scheduled.
	Count int

	// Constraints can be specified at a task group level and apply to
	// all the tasks contained.
	Constraints []*Constraint

	// Tasks are the collection of tasks that this task group needs to run
	Tasks []*Task

	// Meta is used to associate arbitrary metadata with this
	// task group. This is opaque to Nomad.
	Meta map[string]string
}

// Task is a single process typically that is executed as part of a task group.
type Task struct {
	// Name of the task
	Name string

	// Driver is used to control which driver is used
	Driver string

	// Config is provided to the driver to initialize
	Config map[string]string

	// Constraints can be specified at a task level and apply only to
	// the particular task.
	Constraints []*Constraint

	// Resources is the resources needed by this task
	Resources *Resources

	// Meta is used to associate arbitrary metadata with this
	// task. This is opaque to Nomad.
	Meta map[string]string
}

// Constraints are used to restrict placement options in the case of
// a hard constraint, and used to prefer a placement in the case of
// a soft constraint.
type Constraint struct {
	Hard    bool   // Hard or soft constraint
	LTarget string // Left-hand target
	RTarget string // Right-hand target
	Operand string // Constraint operand (<=, <, =, !=, >, >=), contains, near
	Weight  int    // Soft constraints can vary the weight
}

const (
	AllocStatusPending  = "pending"
	AllocStatusInit     = "initializing"
	AllocStatusRunning  = "running"
	AllocStatusComplete = "complete"
	AllocStatusDead     = "dead"
)

// Allocation is used to allocate the placement of a task group to a node.
type Allocation struct {
	// ID of the allocation (UUID)
	ID string

	// NodeID is the node this is being placed on
	NodeID string

	// Job is the parent job of the task group being allocated.
	// This is copied at allocation time to avoid issues if the job
	// definition is updated.
	JobID string
	Job   *Job

	// TaskGroup is the task being allocated to the node
	// This is copied at allocation time to avoid issues if the job
	// definition is updated.
	TaskGroupName string
	TaskGroup     *TaskGroup

	// Resources is the set of resources allocated as part
	// of this allocation of the task group.
	Resources *Resources

	// Metrics associated with this allocation
	Metrics *AllocMetric

	// Status of the allocation
	Status string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// AllocMetric is used to track various metrics while attempting
// to make an allocation. These are used to debug a job, or to better
// understand the pressure within the system.
type AllocMetric struct {
	// NodesEvaluated is the number of nodes that were evaluated
	NodesEvaluated int

	// NodesFiltered is the number of nodes filtered due to
	// a hard constraint
	NodesFiltered int

	// ClassFiltered is the number of nodes filtered by class
	ClassFiltered map[string]int

	// ConstraintFiltered is the number of failures caused by constraint
	ConstraintFiltered map[string]int

	// NodesExhausted is the nubmer of nodes skipped due to being
	// exhausted of at least one resource
	NodesExhausted int

	// ClassExhausted is the number of nodes exhausted by class
	ClassExhausted map[string]int

	// Preemptions is the number of preemptions considered.
	// This indicates a relatively busy fleet if high.
	Preemptions int

	// Scores is the scores of the final few nodes remaining
	// for placement. The top score is typically selected.
	Scores map[string]int

	// AllocationTime is a measure of how long the allocation
	// attempt took. This can affect performance and SLAs.
	AllocationTime time.Duration
}

const (
	EvalStatusPending  = "pending"
	EvalStatusComplete = "complete"
	EvalStatusCanceled = "canceled"
)

// Evaluation is used anytime we need to apply business logic as a result
// of a change to our desired state (job specification) or the emergent state
// (registered nodes). When the inputs change, we need to "evaluate" them,
// potentially taking action (allocation of work) or doing nothing if the state
// of the world does not require it.
type Evaluation struct {
	// ID is a randonly generated UUID used for this evaluation. This
	// is assigned upon the creation of the evaluation.
	ID string

	// Status of the evaluation
	Status string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
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
