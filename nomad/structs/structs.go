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
	EvalUpdateRequestType
	EvalDeleteRequestType
	AllocUpdateRequestType
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

// EvalUpdateRequest is used for upserting evaluations.
type EvalUpdateRequest struct {
	Evals []*Evaluation
	WriteRequest
}

// EvalDeleteRequest is used for deleting an evaluation.
type EvalDeleteRequest struct {
	EvalID string
	WriteRequest
}

// EvalSpecificRequest is used when we just need to specify a target evaluation
type EvalSpecificRequest struct {
	EvalID string
	WriteRequest
}

// EvalAckRequest is used to Ack/Nack a specific evaluation
type EvalAckRequest struct {
	EvalID string
	Token  string
	WriteRequest
}

// EvalDequeueRequest is used when we want to dequeue an evaluation
type EvalDequeueRequest struct {
	Schedulers []string
	Timeout    time.Duration
	WriteRequest
}

// PlanRequest is used to submit an allocation plan to the leader
type PlanRequest struct {
	Plan *Plan
	WriteRequest
}

// AllocUpdateRequest is used to submit changes to allocations, either
// to cause evictions or to assign new allocaitons. Both can be done
// within a single transaction
type AllocUpdateRequest struct {
	// Evict is the list of allocation IDs to evict
	Evict []string

	// Alloc is the list of new allocations to assign
	Alloc []*Allocation
}

// GenericResponse is used to respond to a request where no
// specific response information is needed.
type GenericResponse struct {
	WriteMeta
}

// JobRegisterResponse is used to respond to a job registration
type JobRegisterResponse struct {
	EvalID          string
	EvalCreateIndex uint64
	JobModifyIndex  uint64
	QueryMeta
}

// JobDeregisterResponse is used to respond to a job deregistration
type JobDeregisterResponse struct {
	EvalID          string
	EvalCreateIndex uint64
	JobModifyIndex  uint64
	QueryMeta
}

// NodeUpdateResponse is used to respond to a node update
type NodeUpdateResponse struct {
	EvalIDs         []string
	EvalCreateIndex uint64
	NodeModifyIndex uint64
	QueryMeta
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

// SingleEvalResponse is used to return a single evaluation
type SingleEvalResponse struct {
	Eval *Evaluation
	QueryMeta
}

// EvalDequeueResponse is used to return from a dequeue
type EvalDequeueResponse struct {
	Eval  *Evaluation
	Token string
	QueryMeta
}

// PlanResponse is used to return from a PlanRequest
type PlanResponse struct {
	Result *PlanResult
	WriteMeta
}

const (
	NodeStatusInit  = "initializing"
	NodeStatusReady = "ready"
	NodeStatusMaint = "maintenance"
	NodeStatusDrain = "drain"
	NodeStatusDown  = "down"
)

// ShouldEvaluateNode checks if a given node status should trigger an
// evaluation. Some states don't require any further action.
func ShouldEvaluateNode(status string) bool {
	switch status {
	case NodeStatusInit, NodeStatusReady, NodeStatusMaint:
		return false
	case NodeStatusDrain, NodeStatusDown:
		return true
	default:
		panic(fmt.Sprintf("unhandled node status %s", status))
	}
}

// ValidNodeStatus is used to check if a node status is valid
func ValidNodeStatus(status string) bool {
	switch status {
	case NodeStatusInit, NodeStatusReady,
		NodeStatusMaint, NodeStatusDrain, NodeStatusDown:
		return true
	default:
		return false
	}
}

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
}

// NetIndexByCIDR scans the list of networks for a matching
// CIDR, returning the index. This currently ONLY handles
// an exact match and not a subset CIDR.
func (r *Resources) NetIndexByCIDR(cidr string) int {
	for idx, net := range r.Networks {
		if net.CIDR == cidr {
			return idx
		}
	}
	return -1
}

// Superset checks if one set of resources is a superset
// of another.
func (r *Resources) Superset(other *Resources) bool {
	if r.CPU < other.CPU {
		return false
	}
	if r.MemoryMB < other.MemoryMB {
		return false
	}
	if r.DiskMB < other.DiskMB {
		return false
	}
	if r.IOPS < other.IOPS {
		return false
	}
	for _, net := range r.Networks {
		idx := other.NetIndexByCIDR(net.CIDR)
		if idx >= 0 {
			if net.MBits < other.Networks[idx].MBits {
				return false
			}
		}
	}
	// Check that other does not have a network we are missing
	for _, net := range other.Networks {
		idx := r.NetIndexByCIDR(net.CIDR)
		if idx == -1 {
			return false
		}
	}
	return true
}

// Add adds the resources of the delta to this, potentially
// returning an error if not possible.
func (r *Resources) Add(delta *Resources) error {
	if delta == nil {
		return nil
	}
	r.CPU += delta.CPU
	r.MemoryMB += delta.MemoryMB
	r.DiskMB += delta.DiskMB
	r.IOPS += delta.IOPS

	for _, net := range delta.Networks {
		idx := r.NetIndexByCIDR(net.CIDR)
		if idx == -1 {
			return fmt.Errorf("missing network for CIDR %s", net.CIDR)
		}
		r.Networks[idx].Add(net)
	}
	return nil
}

// NetworkResource is used to represesent available network
// resources
type NetworkResource struct {
	Public        bool   // Is this a public address?
	CIDR          string // CIDR block of addresses
	ReservedPorts []int  // Reserved ports
	MBits         int    // Throughput
}

// Add adds the resources of the delta to this, potentially
// returning an error if not possible.
func (n *NetworkResource) Add(delta *NetworkResource) {
	if len(delta.ReservedPorts) > 0 {
		n.ReservedPorts = append(n.ReservedPorts, delta.ReservedPorts...)
	}
	n.MBits += delta.MBits
}

const (
	// JobTypeSystem is reserved for internal system tasks and is
	// always handled by the SystemScheduler.
	JobTypeSystem  = "system"
	JobTypeService = "service"
	JobTypeBatch   = "batch"
)

const (
	JobStatusPending  = "pending"  // Pending means the job is waiting on scheduling
	JobStatusRunning  = "running"  // Running means the entire job is running
	JobStatusComplete = "complete" // Complete means there was a clean termination
	JobStatusDead     = "dead"     // Dead means there was abnormal termination
)

const (
	// JobMinPriority is the minimum allowed priority
	JobMinPriority = 1

	// JobDefaultPriority is the default priority if not
	// not specified.
	JobDefaultPriority = 50

	// JobMaxPriority is the maximum allowed priority
	JobMaxPriority = 100
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

	// Name is a logical name of the allocation.
	Name string

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

const (
	EvalTriggerJobRegister   = "job-register"
	EvalTriggerJobDeregister = "job-deregister"
	EvalTriggerNodeUpdate    = "node-update"
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

	// Priority is used to control scheduling importance and if this job
	// can preempt other jobs.
	Priority int

	// Type is used to control which schedulers are available to handle
	// this evaluation.
	Type string

	// TriggeredBy is used to give some insight into why this Eval
	// was created. (Job change, node failure, alloc failure, etc).
	TriggeredBy string

	// JobID is the job this evaluation is scoped to. Evalutions cannot
	// be run in parallel for a given JobID, so we serialize on this.
	JobID string

	// JobModifyIndex is the modify index of the job at the time
	// the evaluation was created
	JobModifyIndex uint64

	// NodeID is the node that was affected triggering the evaluation.
	NodeID string

	// NodeModifyIndex is the modify index of the node at the time
	// the evaluation was created
	NodeModifyIndex uint64

	// Status of the evaluation
	Status string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

// ShouldEnqueue checks if a given evaluation should be enqueued
func (e *Evaluation) ShouldEnqueue() bool {
	switch e.Status {
	case EvalStatusPending:
		return true
	case EvalStatusComplete, EvalStatusCanceled:
		return false
	default:
		panic(fmt.Sprintf("unhandled evaluation (%s) status %s", e.ID, e.Status))
	}
}

// MakePlan is used to make a plan from the given evaluation
// for a given Job
func (e *Evaluation) MakePlan(j *Job) *Plan {
	p := &Plan{
		EvalID:         e.ID,
		Priority:       j.Priority,
		AllAtOnce:      j.AllAtOnce,
		NodeEvict:      make(map[string][]string),
		NodeAllocation: make(map[string][]*Allocation),
	}
	return p
}

// Plan is used to submit a commit plan for task allocations. These
// are submitted to the leader which verifies that resources have
// not been overcommitted before admiting the plan.
type Plan struct {
	// EvalID is the evaluation ID this plan is associated with
	EvalID string

	// EvalToken is used to prevent a split-brain processing of
	// an evaluation. There should only be a single scheduler running
	// an Eval at a time, but this could be violated after a leadership
	// transition. This unique token is used to reject plans that are
	// being submitted from a different leader.
	EvalToken string

	// Priority is the priority of the upstream job
	Priority int

	// AllAtOnce is used to control if incremental scheduling of task groups
	// is allowed or if we must do a gang scheduling of the entire job.
	// If this is false, a plan may be partially applied. Otherwise, the
	// entire plan must be able to make progress.
	AllAtOnce bool

	// NodeEvict contains all the evictions for each node. For each node,
	// this is a list of the allocation IDs to evict.
	NodeEvict map[string][]string

	// NodeAllocation contains all the allocations for each node.
	// The evicts must be considered prior to the allocations.
	NodeAllocation map[string][]*Allocation
}

// PlanResult is the result of a plan submitted to the leader.
type PlanResult struct {
	// NodeEvict contains all the evictions that were committed.
	NodeEvict map[string][]string

	// NodeAllocation contains all the allocations that were committed.
	NodeAllocation map[string][]*Allocation

	// RefreshIndex is the index the worker should refresh state up to.
	// This allows all evictions and allocations to be materialized.
	// If any allocations were rejected due to stale data (node state,
	// over committed) this can be used to force a worker refresh.
	RefreshIndex uint64

	// AllocIndex is the Raft index in which the evictions and
	// allocations took place. This is used for the write index.
	AllocIndex uint64
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
