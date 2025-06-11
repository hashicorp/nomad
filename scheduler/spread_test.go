// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/feasible"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// TestSpreadOnLargeCluster exercises potentially quadratic
// performance cases with spread scheduling when we have a large
// number of eligible nodes unless we limit the number that each
// MaxScore attempt considers. By reducing the total from MaxInt, we
// can prevent quadratic performance but then we need this test to
// verify we have satisfactory spread results.
func TestSpreadOnLargeCluster(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name      string
		nodeCount int
		racks     map[string]int
		allocs    int
	}{
		{
			name:      "nodes=10k even racks=100 allocs=500",
			nodeCount: 10000,
			racks:     generateEvenRacks(10000, 100),
			allocs:    500,
		},
		{
			name:      "nodes=10k even racks=100 allocs=50",
			nodeCount: 10000,
			racks:     generateEvenRacks(10000, 100),
			allocs:    50,
		},
		{
			name:      "nodes=10k even racks=10 allocs=500",
			nodeCount: 10000,
			racks:     generateEvenRacks(10000, 10),
			allocs:    500,
		},
		{
			name:      "nodes=10k even racks=10 allocs=50",
			nodeCount: 10000,
			racks:     generateEvenRacks(10000, 10),
			allocs:    500,
		},
		{
			name:      "nodes=10k small uneven racks allocs=500",
			nodeCount: 10000,
			racks:     generateUnevenRacks(t, 10000, 50),
			allocs:    500,
		},
		{
			name:      "nodes=10k small uneven racks allocs=50",
			nodeCount: 10000,
			racks:     generateUnevenRacks(t, 10000, 50),
			allocs:    500,
		},
		{
			name:      "nodes=10k many uneven racks allocs=500",
			nodeCount: 10000,
			racks:     generateUnevenRacks(t, 10000, 500),
			allocs:    500,
		},
		{
			name:      "nodes=10k many uneven racks allocs=50",
			nodeCount: 10000,
			racks:     generateUnevenRacks(t, 10000, 500),
			allocs:    50,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			h := tests.NewHarness(t)
			err := upsertNodes(h, tc.nodeCount, tc.racks)
			must.NoError(t, err)
			job := generateJob(tc.allocs)
			eval, err := upsertJob(h, job)
			must.NoError(t, err)

			start := time.Now()
			err = h.Process(NewServiceScheduler, eval)
			must.NoError(t, err)
			must.LessEq(t, time.Duration(60*time.Second), time.Since(start),
				must.Sprint("time to evaluate exceeded EvalNackTimeout"))

			must.Len(t, 1, h.Plans)
			must.False(t, h.Plans[0].IsNoOp())
			must.NoError(t, validateEqualSpread(h))
		})
	}
}

// generateUnevenRacks creates a map of rack names to a count of nodes
// evenly distributed in those racks
func generateEvenRacks(nodes int, rackCount int) map[string]int {
	racks := map[string]int{}
	for i := 0; i < nodes; i++ {
		racks[fmt.Sprintf("r%d", i%rackCount)]++
	}
	return racks
}

// generateUnevenRacks creates a random map of rack names to a count
// of nodes in that rack
func generateUnevenRacks(t *testing.T, nodes int, rackCount int) map[string]int {
	rackNames := []string{}
	for i := 0; i < rackCount; i++ {
		rackNames = append(rackNames, fmt.Sprintf("r%d", i))
	}

	// print this so that any future test flakes can be more easily
	// reproduced
	seed := time.Now().Unix()
	random := rand.NewSource(seed)
	t.Logf("nodes=%d racks=%d seed=%d\n", nodes, rackCount, seed)

	racks := map[string]int{}
	for i := 0; i < nodes; i++ {
		idx := int(random.Int63()) % len(rackNames)
		racks[rackNames[idx]]++
	}
	return racks
}

// upsertNodes creates a collection of Nodes in the state store,
// distributed among the racks
func upsertNodes(h *tests.Harness, count int, racks map[string]int) error {

	datacenters := []string{"dc-1", "dc-2"}
	rackAssignments := []string{}
	for rack, count := range racks {
		for i := 0; i < count; i++ {
			rackAssignments = append(rackAssignments, rack)
		}
	}

	for i := 0; i < count; i++ {
		node := mock.Node()
		node.Datacenter = datacenters[i%2]
		node.Meta = map[string]string{}
		node.Meta["rack"] = fmt.Sprintf("r%s", rackAssignments[i])
		node.NodeResources.Cpu.CpuShares = 14000
		node.NodeResources.Memory.MemoryMB = 32000
		err := h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateJob(jobSize int) *structs.Job {
	job := mock.Job()
	job.Datacenters = []string{"dc-1", "dc-2"}
	job.Spreads = []*structs.Spread{{Attribute: "${meta.rack}"}}
	job.Constraints = []*structs.Constraint{}
	job.TaskGroups[0].Count = jobSize
	job.TaskGroups[0].Networks = nil
	job.TaskGroups[0].Services = []*structs.Service{}
	job.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      6000,
		MemoryMB: 6000,
	}
	return job
}

func upsertJob(h *tests.Harness, job *structs.Job) (*structs.Evaluation, error) {
	err := h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job)
	if err != nil {
		return nil, err
	}
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	err = h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval})
	if err != nil {
		return nil, err
	}
	return eval, nil
}

// validateEqualSpread compares the resulting plan to the node
// metadata to verify that each group of spread targets has an equal
// distribution.
func validateEqualSpread(h *tests.Harness) error {

	iter, err := h.State.Nodes(nil)
	if err != nil {
		return err
	}
	i := 0
	nodesToRacks := map[string]string{}
	racksToAllocCount := map[string]int{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		node := raw.(*structs.Node)
		rack, ok := node.Meta["rack"]
		if ok {
			nodesToRacks[node.ID] = rack
			racksToAllocCount[rack] = 0
		}
		i++
	}

	// Collapse the count of allocations per node into a list of
	// counts. The results should be clustered within one of each
	// other.
	for nodeID, nodeAllocs := range h.Plans[0].NodeAllocation {
		racksToAllocCount[nodesToRacks[nodeID]] += len(nodeAllocs)
	}
	countSet := map[int]int{}
	for _, count := range racksToAllocCount {
		countSet[count]++
	}

	countSlice := []int{}
	for count := range countSet {
		countSlice = append(countSlice, count)
	}

	switch len(countSlice) {
	case 1:
		return nil
	case 2, 3:
		sort.Ints(countSlice)
		for i := 1; i < len(countSlice); i++ {
			if countSlice[i] != countSlice[i-1]+1 {
				return fmt.Errorf("expected even distributon of allocs to racks, but got:\n%+v", countSet)
			}
		}
		return nil
	}
	return fmt.Errorf("expected even distributon of allocs to racks, but got:\n%+v", countSet)
}

func TestSpreadPanicDowngrade(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	nodes := []*structs.Node{}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		err := h.State.UpsertNode(structs.MsgTypeTestSetup,
			h.NextIndex(), node)
		must.NoError(t, err)
	}

	// job version 1
	// max_parallel = 0, canary = 1, spread != nil, 1 failed alloc

	job1 := mock.Job()
	job1.Spreads = []*structs.Spread{
		{
			Attribute:    "${node.unique.name}",
			Weight:       50,
			SpreadTarget: []*structs.SpreadTarget{},
		},
	}
	job1.Update = structs.UpdateStrategy{
		Stagger:     time.Duration(30 * time.Second),
		MaxParallel: 0,
	}
	job1.Status = structs.JobStatusRunning
	job1.TaskGroups[0].Count = 4
	job1.TaskGroups[0].Update = &structs.UpdateStrategy{
		Stagger:          time.Duration(30 * time.Second),
		MaxParallel:      1,
		HealthCheck:      "checks",
		MinHealthyTime:   time.Duration(30 * time.Second),
		HealthyDeadline:  time.Duration(9 * time.Minute),
		ProgressDeadline: time.Duration(10 * time.Minute),
		AutoRevert:       true,
		Canary:           1,
	}

	job1.Version = 1
	job1.TaskGroups[0].Count = 5
	err := h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1)
	must.NoError(t, err)

	allocs := []*structs.Allocation{}
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		alloc.NodeID = nodes[i].ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy:     pointer.Of(true),
			Timestamp:   time.Now(),
			Canary:      false,
			ModifyIndex: h.NextIndex(),
		}
		if i == 0 {
			alloc.DeploymentStatus.Canary = true
		}
		if i == 1 {
			alloc.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, alloc)
	}
	err = h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs)
	must.NoError(t, err)

	// job version 2
	// max_parallel = 0, canary = 1, spread == nil

	job2 := job1.Copy()
	job2.Version = 2
	job2.Spreads = nil
	err = h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2)
	must.NoError(t, err)

	eval := &structs.Evaluation{
		Namespace:   job2.Namespace,
		ID:          uuid.Generate(),
		Priority:    job2.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job2.ID,
		Status:      structs.EvalStatusPending,
	}
	err = h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval})
	must.NoError(t, err)

	processErr := h.Process(NewServiceScheduler, eval)
	must.NoError(t, processErr, must.Sprintf("..."))
	must.Len(t, 1, h.Plans)
}

func TestSpread_ImplicitTargets(t *testing.T) {

	dcs := []string{"dc1", "dc2", "dc3"}

	setupNodes := func(h *tests.Harness) map[string]string {
		nodesToDcs := map[string]string{}
		var nodes []*feasible.RankedNode

		for i, dc := range dcs {
			for n := 0; n < 4; n++ {
				node := mock.Node()
				node.Datacenter = dc
				must.NoError(t, h.State.UpsertNode(
					structs.MsgTypeTestSetup, uint64(100+i), node))
				nodes = append(nodes, &feasible.RankedNode{Node: node})
				nodesToDcs[node.ID] = node.Datacenter
			}
		}
		return nodesToDcs
	}

	setupJob := func(h *tests.Harness, testCaseSpread *structs.Spread) *structs.Evaluation {
		job := mock.MinJob()
		job.Datacenters = dcs
		job.TaskGroups[0].Count = 12

		job.TaskGroups[0].Spreads = []*structs.Spread{testCaseSpread}
		must.NoError(t, h.State.UpsertJob(
			structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

		eval := &structs.Evaluation{
			Namespace:   structs.DefaultNamespace,
			ID:          uuid.Generate(),
			Priority:    job.Priority,
			TriggeredBy: structs.EvalTriggerJobRegister,
			JobID:       job.ID,
			Status:      structs.EvalStatusPending,
		}
		must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup,
			h.NextIndex(), []*structs.Evaluation{eval}))

		return eval
	}

	testCases := []struct {
		name   string
		spread *structs.Spread
		expect map[string]int
	}{
		{

			name: "empty implicit target",
			spread: &structs.Spread{
				Weight:    100,
				Attribute: "${node.datacenter}",
				SpreadTarget: []*structs.SpreadTarget{
					{
						Value:   "dc1",
						Percent: 50,
					},
				},
			},
			expect: map[string]int{"dc1": 6},
		},
		{
			name: "wildcard implicit target",
			spread: &structs.Spread{
				Weight:    100,
				Attribute: "${node.datacenter}",
				SpreadTarget: []*structs.SpreadTarget{
					{
						Value:   "dc1",
						Percent: 50,
					},
					{
						Value:   "*",
						Percent: 50,
					},
				},
			},
			expect: map[string]int{"dc1": 6},
		},
		{
			name: "explicit targets",
			spread: &structs.Spread{
				Weight:    100,
				Attribute: "${node.datacenter}",
				SpreadTarget: []*structs.SpreadTarget{
					{
						Value:   "dc1",
						Percent: 50,
					},
					{
						Value:   "dc2",
						Percent: 25,
					},
					{
						Value:   "dc3",
						Percent: 25,
					},
				},
			},
			expect: map[string]int{"dc1": 6, "dc2": 3, "dc3": 3},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := tests.NewHarness(t)
			nodesToDcs := setupNodes(h)
			eval := setupJob(h, tc.spread)
			must.NoError(t, h.Process(NewServiceScheduler, eval))
			must.Len(t, 1, h.Plans)

			plan := h.Plans[0]
			must.False(t, plan.IsNoOp())

			dcCounts := map[string]int{}
			for node, allocs := range plan.NodeAllocation {
				dcCounts[nodesToDcs[node]] += len(allocs)
			}
			for dc, expectVal := range tc.expect {
				// not using test.MapEqual here because we have incomplete
				// expectations for the implicit DCs on some tests.
				test.Eq(t, expectVal, dcCounts[dc],
					test.Sprintf("expected %d in %q", expectVal, dc))
			}

		})
	}
}
