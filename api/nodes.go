package api

import (
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Nodes is used to query node-related API endpoints
type Nodes struct {
	client *Client
}

// Nodes returns a handle on the node endpoints.
func (c *Client) Nodes() *Nodes {
	return &Nodes{client: c}
}

// List is used to list out all of the nodes
func (n *Nodes) List(q *QueryOptions) ([]*NodeListStub, *QueryMeta, error) {
	var resp NodeIndexSort
	qm, err := n.client.query("/v1/nodes", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(resp)
	return resp, qm, nil
}

func (n *Nodes) PrefixList(prefix string) ([]*NodeListStub, *QueryMeta, error) {
	return n.List(&QueryOptions{Prefix: prefix})
}

// Info is used to query a specific node by its ID.
func (n *Nodes) Info(nodeID string, q *QueryOptions) (*Node, *QueryMeta, error) {
	var resp Node
	qm, err := n.client.query("/v1/node/"+nodeID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// NodeUpdateDrainRequest is used to update the drain specification for a node.
type NodeUpdateDrainRequest struct {
	// NodeID is the node to update the drain specification for.
	NodeID string

	// DrainSpec is the drain specification to set for the node. A nil DrainSpec
	// will disable draining.
	DrainSpec *DrainSpec
}

// UpdateDrain is used to update the drain strategy for a given node.
func (n *Nodes) UpdateDrain(nodeID string, spec *DrainSpec, q *WriteOptions) (*WriteMeta, error) {
	req := &NodeUpdateDrainRequest{
		NodeID:    nodeID,
		DrainSpec: spec,
	}

	wm, err := n.client.write("/v1/node/"+nodeID+"/drain", req, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// NodeUpdateEligibilityRequest is used to update the drain specification for a node.
type NodeUpdateEligibilityRequest struct {
	// NodeID is the node to update the drain specification for.
	NodeID      string
	Eligibility string
}

// ToggleEligibility is used to update the scheduling eligibility of the node
func (n *Nodes) ToggleEligibility(nodeID string, eligible bool, q *WriteOptions) (*WriteMeta, error) {
	e := structs.NodeSchedulingEligible
	if !eligible {
		e = structs.NodeSchedulingIneligible
	}

	req := &NodeUpdateEligibilityRequest{
		NodeID:      nodeID,
		Eligibility: e,
	}

	wm, err := n.client.write("/v1/node/"+nodeID+"/eligibility", req, nil, q)
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// Allocations is used to return the allocations associated with a node.
func (n *Nodes) Allocations(nodeID string, q *QueryOptions) ([]*Allocation, *QueryMeta, error) {
	var resp []*Allocation
	qm, err := n.client.query("/v1/node/"+nodeID+"/allocations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(AllocationSort(resp))
	return resp, qm, nil
}

// ForceEvaluate is used to force-evaluate an existing node.
func (n *Nodes) ForceEvaluate(nodeID string, q *WriteOptions) (string, *WriteMeta, error) {
	var resp nodeEvalResponse
	wm, err := n.client.write("/v1/node/"+nodeID+"/evaluate", nil, &resp, q)
	if err != nil {
		return "", nil, err
	}
	return resp.EvalID, wm, nil
}

func (n *Nodes) Stats(nodeID string, q *QueryOptions) (*HostStats, error) {
	var resp HostStats
	path := fmt.Sprintf("/v1/client/stats?node_id=%s", nodeID)
	if _, err := n.client.query(path, &resp, q); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (n *Nodes) GC(nodeID string, q *QueryOptions) error {
	var resp struct{}
	path := fmt.Sprintf("/v1/client/gc?node_id=%s", nodeID)
	_, err := n.client.query(path, &resp, q)
	return err
}

// TODO Add tests
func (n *Nodes) GcAlloc(allocID string, q *QueryOptions) error {
	var resp struct{}
	path := fmt.Sprintf("/v1/client/allocation/%s/gc", allocID)
	_, err := n.client.query(path, &resp, q)
	return err
}

// Node is used to deserialize a node entry.
type Node struct {
	ID                    string
	Datacenter            string
	Name                  string
	HTTPAddr              string
	TLSEnabled            bool
	Attributes            map[string]string
	Resources             *Resources
	Reserved              *Resources
	Links                 map[string]string
	Meta                  map[string]string
	NodeClass             string
	Drain                 bool
	DrainStrategy         *DrainStrategy
	SchedulingEligibility string
	Status                string
	StatusDescription     string
	StatusUpdatedAt       int64
	Events                []*NodeEvent
	CreateIndex           uint64
	ModifyIndex           uint64
}

// DrainStrategy describes a Node's drain behavior.
type DrainStrategy struct {
	// DrainSpec is the user declared drain specification
	DrainSpec
}

// DrainSpec describes a Node's drain behavior.
type DrainSpec struct {
	// Deadline is the duration after StartTime when the remaining
	// allocations on a draining Node should be told to stop.
	Deadline time.Duration

	// IgnoreSystemJobs allows systems jobs to remain on the node even though it
	// has been marked for draining.
	IgnoreSystemJobs bool
}

const (
	NodeEventSubsystemDrain     = "Drain"
	NodeEventSubsystemDriver    = "Driver"
	NodeEventSubsystemHeartbeat = "Heartbeat"
	NodeEventSubsystemCluster   = "Cluster"
)

// NodeEvent is a single unit representing a node’s state change
type NodeEvent struct {
	Message     string
	Subsystem   string
	Details     map[string]string
	Timestamp   int64
	CreateIndex uint64
}

// HostStats represents resource usage stats of the host running a Nomad client
type HostStats struct {
	Memory           *HostMemoryStats
	CPU              []*HostCPUStats
	DiskStats        []*HostDiskStats
	Uptime           uint64
	CPUTicksConsumed float64
}

type HostMemoryStats struct {
	Total     uint64
	Available uint64
	Used      uint64
	Free      uint64
}

type HostCPUStats struct {
	CPU    string
	User   float64
	System float64
	Idle   float64
}

type HostDiskStats struct {
	Device            string
	Mountpoint        string
	Size              uint64
	Used              uint64
	Available         uint64
	UsedPercent       float64
	InodesUsedPercent float64
}

// NodeListStub is a subset of information returned during
// node list operations.
type NodeListStub struct {
	Address               string
	ID                    string
	Datacenter            string
	Name                  string
	NodeClass             string
	Version               string
	Drain                 bool
	SchedulingEligibility string
	Status                string
	StatusDescription     string
	CreateIndex           uint64
	ModifyIndex           uint64
}

// NodeIndexSort reverse sorts nodes by CreateIndex
type NodeIndexSort []*NodeListStub

func (n NodeIndexSort) Len() int {
	return len(n)
}

func (n NodeIndexSort) Less(i, j int) bool {
	return n[i].CreateIndex > n[j].CreateIndex
}

func (n NodeIndexSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// nodeEvalResponse is used to decode a force-eval.
type nodeEvalResponse struct {
	EvalID string
}

// AllocationSort reverse sorts allocs by CreateIndex.
type AllocationSort []*Allocation

func (a AllocationSort) Len() int {
	return len(a)
}

func (a AllocationSort) Less(i, j int) bool {
	return a[i].CreateIndex > a[j].CreateIndex
}

func (a AllocationSort) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
