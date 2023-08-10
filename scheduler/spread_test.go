// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestSpreadIterator_SingleAttribute(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	dcs := []string{"dc1", "dc2", "dc1", "dc1"}
	var nodes []*RankedNode

	// Add these nodes to the state store
	for i, dc := range dcs {
		node := mock.Node()
		node.Datacenter = dc
		if err := state.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
		nodes = append(nodes, &RankedNode{Node: node})
	}

	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	tg := job.TaskGroups[0]
	job.TaskGroups[0].Count = 10
	// add allocs to nodes in dc1
	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[2].Node.ID,
		},
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	// Create spread target of 80% in dc1
	// Implicitly, this means 20% in dc2
	spread := &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "dc1",
				Percent: 80,
			},
		},
	}
	tg.Spreads = []*structs.Spread{spread}
	spreadIter := NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, spreadIter)

	out := collectRanked(scoreNorm)

	// Expect nodes in dc1 with existing allocs to get a boost
	// Boost should be ((desiredCount-actual)/desired)*spreadWeight
	// For this test, that becomes dc1 = ((8-3)/8 ) = 0.5, and dc2=(2-1)/2
	expectedScores := map[string]float64{
		"dc1": 0.625,
		"dc2": 0.5,
	}
	for _, rn := range out {
		require.Equal(t, expectedScores[rn.Node.Datacenter], rn.FinalScore)
	}

	// Update the plan to add more allocs to nodes in dc1
	// After this step there are enough allocs to meet the desired count in dc1
	ctx.plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: "bbb",
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
	}
	ctx.plan.NodeAllocation[nodes[3].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[3].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[3].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[3].Node.ID,
		},
	}

	// Reset the scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)
	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)
	out = collectRanked(scoreNorm)

	// Expect nodes in dc2 with existing allocs to get a boost
	// DC1 nodes are not boosted because there are enough allocs to meet
	// the desired count
	expectedScores = map[string]float64{
		"dc1": 0,
		"dc2": 0.5,
	}
	for _, rn := range out {
		require.Equal(t, expectedScores[rn.Node.Datacenter], rn.FinalScore)
	}
}

func TestSpreadIterator_MultipleAttributes(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	dcs := []string{"dc1", "dc2", "dc1", "dc1"}
	rack := []string{"r1", "r1", "r2", "r2"}
	var nodes []*RankedNode

	// Add these nodes to the state store
	for i, dc := range dcs {
		node := mock.Node()
		node.Datacenter = dc
		node.Meta["rack"] = rack[i]
		if err := state.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
		nodes = append(nodes, &RankedNode{Node: node})
	}

	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	tg := job.TaskGroups[0]
	job.TaskGroups[0].Count = 10
	// add allocs to nodes in dc1
	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[2].Node.ID,
		},
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	spread1 := &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "dc1",
				Percent: 60,
			},
			{
				Value:   "dc2",
				Percent: 40,
			},
		},
	}

	spread2 := &structs.Spread{
		Weight:    50,
		Attribute: "${meta.rack}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "r1",
				Percent: 40,
			},
			{
				Value:   "r2",
				Percent: 60,
			},
		},
	}

	tg.Spreads = []*structs.Spread{spread1, spread2}
	spreadIter := NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, spreadIter)

	out := collectRanked(scoreNorm)

	// Score comes from combining two different spread factors
	// Second node should have the highest score because it has no allocs and its in dc2/r1
	expectedScores := map[string]float64{
		nodes[0].Node.ID: 0.500,
		nodes[1].Node.ID: 0.667,
		nodes[2].Node.ID: 0.556,
		nodes[3].Node.ID: 0.556,
	}
	for _, rn := range out {
		require.Equal(t, fmt.Sprintf("%.3f", expectedScores[rn.Node.ID]), fmt.Sprintf("%.3f", rn.FinalScore))
	}

}

func TestSpreadIterator_EvenSpread(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	dcs := []string{"dc1", "dc2", "dc1", "dc2", "dc1", "dc2", "dc2", "dc1", "dc1", "dc1"}
	var nodes []*RankedNode

	// Add these nodes to the state store
	for i, dc := range dcs {
		node := mock.Node()
		node.Datacenter = dc
		if err := state.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
		nodes = append(nodes, &RankedNode{Node: node})
	}

	static := NewStaticRankIterator(ctx, nodes)
	job := mock.Job()
	tg := job.TaskGroups[0]
	job.TaskGroups[0].Count = 10

	// Configure even spread across node.datacenter
	spread := &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
	}
	tg.Spreads = []*structs.Spread{spread}
	spreadIter := NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, spreadIter)

	out := collectRanked(scoreNorm)

	// Nothing placed so both dc nodes get 0 as the score
	expectedScores := map[string]float64{
		"dc1": 0,
		"dc2": 0,
	}
	for _, rn := range out {
		require.Equal(t, fmt.Sprintf("%.3f", expectedScores[rn.Node.Datacenter]), fmt.Sprintf("%.3f", rn.FinalScore))

	}

	// Update the plan to add allocs to nodes in dc1
	// After this step dc2 nodes should get boosted
	ctx.plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].Node.ID,
		},
	}
	ctx.plan.NodeAllocation[nodes[2].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[2].Node.ID,
		},
	}

	// Reset the scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)
	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)
	out = collectRanked(scoreNorm)

	// Expect nodes in dc2 with existing allocs to get a boost
	// dc1 nodes are penalized because they have allocs
	expectedScores = map[string]float64{
		"dc1": -1,
		"dc2": 1,
	}
	for _, rn := range out {
		require.Equal(t, expectedScores[rn.Node.Datacenter], rn.FinalScore)
	}

	// Update the plan to add more allocs to nodes in dc2
	// After this step dc1 nodes should get boosted
	ctx.plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[1].Node.ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[1].Node.ID,
		},
	}
	ctx.plan.NodeAllocation[nodes[3].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[3].Node.ID,
		},
	}

	// Reset the scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)
	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)
	out = collectRanked(scoreNorm)

	// Expect nodes in dc2 to be penalized because there are 3 allocs there now
	// dc1 nodes are boosted because that has 2 allocs
	expectedScores = map[string]float64{
		"dc1": 0.5,
		"dc2": -0.5,
	}
	for _, rn := range out {
		require.Equal(t, fmt.Sprintf("%3.3f", expectedScores[rn.Node.Datacenter]), fmt.Sprintf("%3.3f", rn.FinalScore))
	}

	// Add another node in dc3
	node := mock.Node()
	node.Datacenter = "dc3"
	if err := state.UpsertNode(structs.MsgTypeTestSetup, uint64(1111), node); err != nil {
		t.Fatalf("failed to upsert node: %v", err)
	}
	nodes = append(nodes, &RankedNode{Node: node})

	// Add another alloc to dc1, now its count matches dc2
	ctx.plan.NodeAllocation[nodes[4].Node.ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[4].Node.ID,
		},
	}

	// Reset scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)
	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)
	out = collectRanked(scoreNorm)

	// Expect dc1 and dc2 to be penalized because they have 3 allocs
	// dc3 should get a boost because it has 0 allocs
	expectedScores = map[string]float64{
		"dc1": -1,
		"dc2": -1,
		"dc3": 1,
	}
	for _, rn := range out {
		require.Equal(t, fmt.Sprintf("%.3f", expectedScores[rn.Node.Datacenter]), fmt.Sprintf("%.3f", rn.FinalScore))
	}

}

// Test scenarios where the spread iterator sets maximum penalty (-1.0)
func TestSpreadIterator_MaxPenalty(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	var nodes []*RankedNode

	// Add nodes in dc3 to the state store
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.Datacenter = "dc3"
		if err := state.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
		nodes = append(nodes, &RankedNode{Node: node})
	}

	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	tg := job.TaskGroups[0]
	job.TaskGroups[0].Count = 5

	// Create spread target of 80% in dc1
	// and 20% in dc2
	spread := &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "dc1",
				Percent: 80,
			},
			{
				Value:   "dc2",
				Percent: 20,
			},
		},
	}
	tg.Spreads = []*structs.Spread{spread}
	spreadIter := NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, spreadIter)

	out := collectRanked(scoreNorm)

	// All nodes are in dc3 so score should be -1
	for _, rn := range out {
		require.Equal(t, -1.0, rn.FinalScore)
	}

	// Reset scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}

	// Create spread on attribute that doesn't exist on any nodes
	spread = &structs.Spread{
		Weight:    100,
		Attribute: "${meta.foo}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "bar",
				Percent: 80,
			},
			{
				Value:   "baz",
				Percent: 20,
			},
		},
	}

	tg.Spreads = []*structs.Spread{spread}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)
	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)
	out = collectRanked(scoreNorm)

	// All nodes don't have the spread attribute so score should be -1
	for _, rn := range out {
		require.Equal(t, -1.0, rn.FinalScore)
	}

}

func TestSpreadIterator_NoInfinity(t *testing.T) {
	ci.Parallel(t)

	store, ctx := testContext(t)
	var nodes []*RankedNode

	// Add 3 nodes in different DCs to the state store
	for i := 1; i < 4; i++ {
		node := mock.Node()
		node.Datacenter = fmt.Sprintf("dc%d", i)
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node))
		nodes = append(nodes, &RankedNode{Node: node})
	}

	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	tg := job.TaskGroups[0]
	job.TaskGroups[0].Count = 8

	// Create spread target of 50% in dc1, 50% in dc2, and 0% in the implicit target
	spread := &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "dc1",
				Percent: 50,
			},
			{
				Value:   "dc2",
				Percent: 50,
			},
			{
				Value:   "*",
				Percent: 0,
			},
		},
	}
	tg.Spreads = []*structs.Spread{spread}
	spreadIter := NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, spreadIter)

	out := collectRanked(scoreNorm)

	// Scores should be even between dc1 and dc2 nodes, without an -Inf on dc3
	must.Len(t, 3, out)
	test.Eq(t, 0.75, out[0].FinalScore)
	test.Eq(t, 0.75, out[1].FinalScore)
	test.Eq(t, -1, out[2].FinalScore)

	// Reset scores
	for _, node := range nodes {
		node.Scores = nil
		node.FinalScore = 0
	}

	// Create very unbalanced spread target to force large negative scores
	spread = &structs.Spread{
		Weight:    100,
		Attribute: "${node.datacenter}",
		SpreadTarget: []*structs.SpreadTarget{
			{
				Value:   "dc1",
				Percent: 99,
			},
			{
				Value:   "dc2",
				Percent: 1,
			},
			{
				Value:   "*",
				Percent: 0,
			},
		},
	}
	tg.Spreads = []*structs.Spread{spread}
	static = NewStaticRankIterator(ctx, nodes)
	spreadIter = NewSpreadIterator(ctx, static)
	spreadIter.SetJob(job)
	spreadIter.SetTaskGroup(tg)

	scoreNorm = NewScoreNormalizationIterator(ctx, spreadIter)

	out = collectRanked(scoreNorm)

	// Scores should heavily favor dc1, with an -Inf on dc3
	must.Len(t, 3, out)
	desired := 8 * 0.99 // 8 allocs * 99%
	test.Eq(t, (desired-1)/desired, out[0].FinalScore)
	test.Eq(t, -11.5, out[1].FinalScore)
	test.LessEq(t, out[1].FinalScore, out[2].FinalScore,
		test.Sprintf("expected implicit dc3 to be <= dc2"))
}

func Test_evenSpreadScoreBoost(t *testing.T) {
	ci.Parallel(t)

	pset := &propertySet{
		existingValues: map[string]uint64{},
		proposedValues: map[string]uint64{
			"dc2": 1,
			"dc1": 1,
			"dc3": 1,
		},
		clearedValues: map[string]uint64{
			"dc2": 1,
			"dc3": 1,
		},
		targetAttribute: "${node.datacenter}",
		targetValues:    &set.Set[string]{},
	}

	opt := &structs.Node{
		Datacenter: "dc2",
	}
	boost := evenSpreadScoreBoost(pset, opt)
	require.False(t, math.IsInf(boost, 1))
	require.Equal(t, 1.0, boost)
}

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
			h := NewHarness(t)
			err := upsertNodes(h, tc.nodeCount, tc.racks)
			require.NoError(t, err)
			job := generateJob(tc.allocs)
			eval, err := upsertJob(h, job)
			require.NoError(t, err)

			start := time.Now()
			err = h.Process(NewServiceScheduler, eval)
			require.NoError(t, err)
			require.LessOrEqual(t, time.Since(start), time.Duration(60*time.Second),
				"time to evaluate exceeded EvalNackTimeout")

			require.Len(t, h.Plans, 1)
			require.False(t, h.Plans[0].IsNoOp())
			require.NoError(t, validateEqualSpread(h))
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
func upsertNodes(h *Harness, count int, racks map[string]int) error {

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

func upsertJob(h *Harness, job *structs.Job) (*structs.Evaluation, error) {
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
func validateEqualSpread(h *Harness) error {

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

	h := NewHarness(t)

	nodes := []*structs.Node{}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		err := h.State.UpsertNode(structs.MsgTypeTestSetup,
			h.NextIndex(), node)
		require.NoError(t, err)
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
	require.NoError(t, err)

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
	require.NoError(t, err)

	// job version 2
	// max_parallel = 0, canary = 1, spread == nil

	job2 := job1.Copy()
	job2.Version = 2
	job2.Spreads = nil
	err = h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2)
	require.NoError(t, err)

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
	require.NoError(t, err)

	processErr := h.Process(NewServiceScheduler, eval)
	require.NoError(t, processErr, "failed to process eval")
	require.Len(t, h.Plans, 1)
}

func TestSpread_ImplicitTargets(t *testing.T) {

	dcs := []string{"dc1", "dc2", "dc3"}

	setupNodes := func(h *Harness) map[string]string {
		nodesToDcs := map[string]string{}
		var nodes []*RankedNode

		for i, dc := range dcs {
			for n := 0; n < 4; n++ {
				node := mock.Node()
				node.Datacenter = dc
				must.NoError(t, h.State.UpsertNode(
					structs.MsgTypeTestSetup, uint64(100+i), node))
				nodes = append(nodes, &RankedNode{Node: node})
				nodesToDcs[node.ID] = node.Datacenter
			}
		}
		return nodesToDcs
	}

	setupJob := func(h *Harness, testCaseSpread *structs.Spread) *structs.Evaluation {
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
			h := NewHarness(t)
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
