package api

import (
	"sort"
	"time"
)

// Allocations is used to query the alloc-related endpoints.
type Allocations struct {
	client *Client
}

// Allocations returns a handle on the allocs endpoints.
func (c *Client) Allocations() *Allocations {
	return &Allocations{client: c}
}

// List returns a list of all of the allocations.
func (a *Allocations) List(q *QueryOptions) ([]*AllocationListStub, *QueryMeta, error) {
	var resp []*AllocationListStub
	qm, err := a.client.query("/v1/allocations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(AllocIndexSort(resp))
	return resp, qm, nil
}

// Info is used to retrieve a single allocation.
func (a *Allocations) Info(allocID string, q *QueryOptions) (*Allocation, *QueryMeta, error) {
	var resp Allocation
	qm, err := a.client.query("/v1/allocation/"+allocID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

// Allocation is used for serialization of allocations.
type Allocation struct {
	ID                 string
	EvalID             string
	Name               string
	NodeID             string
	JobID              string
	Job                *Job
	TaskGroup          string
	Resources          *Resources
	TaskResources      map[string]*Resources
	Metrics            *AllocationMetric
	DesiredStatus      string
	DesiredDescription string
	ClientStatus       string
	ClientDescription  string
	CreateIndex        uint64
	ModifyIndex        uint64
}

// AllocationMetric is used to deserialize allocation metrics.
type AllocationMetric struct {
	NodesEvaluated     int
	NodesFiltered      int
	ClassFiltered      map[string]int
	ConstraintFiltered map[string]int
	NodesExhausted     int
	ClassExhausted     map[string]int
	DimensionExhausted map[string]int
	Scores             map[string]float64
	AllocationTime     time.Duration
	CoalescedFailures  int
}

// AllocationListStub is used to return a subset of an allocation
// during list operations.
type AllocationListStub struct {
	ID                 string
	EvalID             string
	Name               string
	NodeID             string
	JobID              string
	TaskGroup          string
	DesiredStatus      string
	DesiredDescription string
	ClientStatus       string
	ClientDescription  string
	CreateIndex        uint64
	ModifyIndex        uint64
}

// AllocIndexSort reverse sorts allocs by CreateIndex.
type AllocIndexSort []*AllocationListStub

func (a AllocIndexSort) Len() int {
	return len(a)
}

func (a AllocIndexSort) Less(i, j int) bool {
	return a[i].CreateIndex > a[j].CreateIndex
}

func (a AllocIndexSort) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
