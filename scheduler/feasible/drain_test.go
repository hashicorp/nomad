// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package feasible

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestDrainChecker_Feasible(t *testing.T) {
	ci.Parallel(t)
	now := time.Now()
	for _, tc := range []struct {
		name     string
		mutate   func(*structs.Node, *structs.Job)
		feasible bool
		reason   string
	}{
		{
			name: "non-draining node accepts unbounded service job",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy = nil
				j.Type = structs.JobTypeService
				j.TaskGroups[0].MaxRunDuration = nil
			},
			feasible: true,
		},
		{
			name: "ordinary drain rejects placement",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.DurationAware = false
			},
			reason: "node drain not duration-aware",
		},
		{name: "bounded batch fits drain budget", feasible: true},
		{
			name: "bounded sysbatch fits drain budget",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.Type = structs.JobTypeSysBatch
			},
			feasible: true,
		},
		{
			name: "service job cannot backfill",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.Type = structs.JobTypeService
			},
			reason: "node drain time budget",
		},
		{
			name: "unbounded batch cannot backfill",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.TaskGroups[0].MaxRunDuration = nil
			},
			reason: "node drain time budget",
		},
		{
			name: "runtime shutdown and buffer exactly reach deadline",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.ForceDeadline = now.Add(100 * time.Second)
			},
			reason: "node drain time budget",
		},
		{
			name: "buffer reduces available runtime",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.BackfillBuffer = time.Minute
			},
			reason: "node drain time budget",
		},
		{
			name: "shutdown reduces available runtime",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.TaskGroups[0].Tasks[0].KillTimeout = time.Minute
			},
			reason: "node drain time budget",
		},
		{
			name: "expired drain deadline",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.ForceDeadline = now.Add(-time.Second)
			},
			reason: "node drain time budget",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := MockContext(t)
			job := mock.BatchJob()
			tg := job.TaskGroups[0]
			tg.MaxRunDuration = new(time.Minute)
			tg.Tasks[0].KillTimeout = 10 * time.Second
			node := mock.Node()
			node.DrainStrategy = &structs.DrainStrategy{
				DrainSpec: structs.DrainSpec{
					DurationAware:  true,
					BackfillBuffer: 30 * time.Second,
				},
				ForceDeadline: now.Add(2 * time.Minute),
			}
			if tc.mutate != nil {
				tc.mutate(node, job)
			}
			checker := NewDrainChecker(ctx)
			checker.SetJob(job)
			checker.SetTaskGroup(tg, nil, now)
			must.Eq(t, tc.feasible, checker.Feasible(node))
			if tc.feasible {
				must.Zero(t, ctx.Metrics().NodesFiltered)
			} else {
				must.One(t, ctx.Metrics().NodesFiltered)
				must.Eq(t, map[string]int{tc.reason: 1}, ctx.Metrics().ConstraintFiltered)
			}
		})
	}
}

func TestDrainChecker_ExistingAllocationStartTime(t *testing.T) {
	ci.Parallel(t)
	_, ctx := MockContext(t)
	now := time.Now()
	job := mock.BatchJob()
	tg := job.TaskGroups[0]
	tg.MaxRunDuration = new(time.Minute)
	tg.Tasks[0].KillTimeout = 10 * time.Second
	node := mock.Node()
	node.DrainStrategy = &structs.DrainStrategy{
		DrainSpec:     structs.DrainSpec{DurationAware: true},
		ForceDeadline: now.Add(70 * time.Second),
	}
	checker := NewDrainChecker(ctx)
	checker.SetJob(job)

	// A new allocation does not fit after accounting for shutdown and the
	// default buffer, but an existing allocation's runtime starts at creation.
	checker.SetTaskGroup(tg, nil, now)
	must.False(t, checker.Feasible(node))
	existing := &structs.Allocation{CreateTime: now.Add(-time.Minute).UnixNano()}
	checker.SetTaskGroup(tg, existing, now)
	must.True(t, checker.Feasible(node))
}

func TestGenericStack_DurationAwareDrainDisablesPreemption(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name          string
		draining      bool
		spareCapacity bool
		preempt       bool
		placed        bool
		preempted     bool
	}{
		{
			name:    "ordinary node preempts lower-priority allocation",
			preempt: true, placed: true, preempted: true,
		},
		{
			name:     "draining node cannot preempt despite eligible runtime",
			draining: true, preempt: true,
		},
		{
			name:     "draining node backfills spare capacity without preemption",
			draining: true, spareCapacity: true, preempt: true, placed: true,
		},
		{name: "ordinary node respects disabled preemption"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, ctx := MockContext(t)
			node := mock.Node()
			if tc.draining {
				node.DrainStrategy = &structs.DrainStrategy{
					DrainSpec:     structs.DrainSpec{DurationAware: true},
					ForceDeadline: time.Now().Add(10 * time.Minute),
				}
			}
			job := mock.BatchJob()
			job.Priority = 100
			tg := job.TaskGroups[0]
			tg.MaxRunDuration = new(time.Minute)

			lowerPriorityJob := mock.BatchJob()
			lowerPriorityJob.Priority = 10
			existing := mock.MinAllocForJob(lowerPriorityJob)
			existing.NodeID = node.ID
			if !tc.spareCapacity {
				// Reclaiming this allocation's memory is the only way to place
				// the new job; its priority makes it an eligible preemption victim.
				existing.AllocatedResources.Tasks[existing.TaskGroup].Memory.MemoryMB =
					node.NodeResources.Memory.MemoryMB - node.ReservedResources.Memory.MemoryMB
			}
			must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, lowerPriorityJob))
			must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{existing}))

			checker := NewDrainChecker(ctx)
			checker.SetJob(job)
			checker.SetTaskGroup(tg, nil, time.Now())
			must.True(t, checker.Feasible(node))

			stack := NewGenericStack(true, ctx)
			stack.SetNodes([]*structs.Node{node})
			stack.SetJob(job)
			option := stack.Select(tg, &SelectOptions{Preempt: tc.preempt})
			// Resource exhaustion, rather than drain feasibility, must reject
			// the full draining node even with preemption explicitly enabled.
			must.Zero(t, ctx.Metrics().NodesFiltered)
			if !tc.placed {
				must.Nil(t, option)
				must.One(t, ctx.Metrics().NodesExhausted)
				return
			}
			must.NotNil(t, option)
			must.Eq(t, node.ID, option.Node.ID)
			if tc.preempted {
				must.Len(t, 1, option.PreemptedAllocs)
				must.Eq(t, existing.ID, option.PreemptedAllocs[0].ID)
			} else {
				must.Len(t, 0, option.PreemptedAllocs)
			}
		})
	}
}

func TestGenericStack_DrainAffinity(t *testing.T) {
	ci.Parallel(t)

	for _, tc := range []struct {
		name     string
		mutate   func(*structs.Node, *structs.Job)
		backfill bool
	}{
		{name: "small workload prefers backfill over perfect binpack", backfill: true},
		{
			name: "backfill outweighs explicit node anti-affinity",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.Affinities = []*structs.Affinity{{
					LTarget: "${node.unique.id}",
					RTarget: n.ID,
					Operand: "=",
					Weight:  -100,
				}}
			},
			backfill: true,
		},
		{
			name: "runtime exceeds drain deadline",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.TaskGroups[0].MaxRunDuration = new(20 * time.Minute)
			},
		},
		{
			name: "buffer consumes remaining budget",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.BackfillBuffer = 10 * time.Minute
			},
		},
		{
			name: "shutdown exceeds remaining budget",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.TaskGroups[0].Tasks[0].KillTimeout = 10 * time.Minute
			},
		},
		{
			name: "unbounded batch job",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.TaskGroups[0].MaxRunDuration = nil
			},
		},
		{
			name: "service job",
			mutate: func(n *structs.Node, j *structs.Job) {
				j.Type = structs.JobTypeService
			},
		},
		{
			name: "ordinary drain",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.DrainStrategy.DurationAware = false
			},
		},
		{
			name: "draining node lacks resources",
			mutate: func(n *structs.Node, j *structs.Job) {
				n.ReservedResources.Cpu.CpuShares = n.NodeResources.Cpu.CpuShares
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ctx := MockContext(t)
			job := mock.BatchJob()
			tg := job.TaskGroups[0]
			tg.MaxRunDuration = new(time.Minute)

			draining := mock.Node()
			draining.DrainStrategy = &structs.DrainStrategy{
				DrainSpec: structs.DrainSpec{
					DurationAware:  true,
					BackfillBuffer: time.Minute,
				},
				ForceDeadline: time.Now().Add(10 * time.Minute),
			}
			if tc.mutate != nil {
				tc.mutate(draining, job)
			}

			// One lightly loaded draining node competes with many ordinary nodes
			// that would be perfectly packed by this small task group.
			nodes := []*structs.Node{draining}
			for range 32 {
				n := mock.Node()
				n.ReservedResources.Cpu.CpuShares = n.NodeResources.Cpu.CpuShares - 100
				n.ReservedResources.Memory.MemoryMB = n.NodeResources.Memory.MemoryMB - 100
				nodes = append(nodes, n)
			}
			stack := NewGenericStack(job.Type == structs.JobTypeBatch, ctx)
			stack.SetNodes(nodes)
			stack.SetJob(job)

			// Repeated selection must keep considering the draining node even
			// as the ordinary candidate iterator advances between placements.
			for range 4 {
				option := stack.Select(tg, &SelectOptions{})
				must.NotNil(t, option)
				if tc.backfill {
					must.Eq(t, draining.ID, option.Node.ID)
					must.True(t, option.FinalScore > 1)
				} else {
					must.True(t, option.Node.DrainStrategy == nil)
				}
			}
		})
	}
}

func TestDrainAffinityIterator_TimeBudget(t *testing.T) {
	ci.Parallel(t)
	_, ctx := MockContext(t)
	now := time.Now()
	job := mock.BatchJob()
	tg := job.TaskGroups[0]
	tg.MaxRunDuration = new(time.Minute)

	// At the exact runtime + shutdown + buffer boundary there is no backfill
	// affinity. A node with additional headroom should beat a perfect binpack.
	boundary := mock.Node()
	boundary.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			DurationAware:  true,
			BackfillBuffer: time.Minute,
		},
		ForceDeadline: now.Add(*tg.MaxRunDuration + tg.MaxShutdown() + time.Minute),
	}
	eligible := boundary.Copy()
	eligible.DrainStrategy.ForceDeadline = eligible.DrainStrategy.ForceDeadline.Add(time.Second)
	ordinary := mock.Node()
	source := NewStaticRankIterator(ctx, []*RankedNode{
		{Node: boundary, Scores: []float64{1}},
		{Node: eligible, Scores: []float64{0}},
		{Node: ordinary, Scores: []float64{1}},
	})
	drain := NewDrainChecker(ctx)
	drain.SetJob(job)
	drain.SetTaskGroup(tg, nil, now)
	iter := NewScoreNormalizationIterator(ctx, NewDrainAffinityIterator(ctx, source, drain))
	options := collectRanked(iter)
	must.Eq(t, 1.0, options[0].FinalScore)
	must.True(t, options[1].FinalScore > options[0].FinalScore)
	must.Eq(t, 1.0, options[2].FinalScore)
}

func TestDrainPriorityIterator_RotatesDrainingCandidates(t *testing.T) {
	ci.Parallel(t)
	_, ctx := MockContext(t)
	draining := []*structs.Node{mock.Node(), mock.Node(), mock.Node()}
	for _, node := range draining {
		node.DrainStrategy = &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{DurationAware: true},
		}
	}
	regular := []*structs.Node{mock.Node(), mock.Node()}
	iter := NewDrainPriorityIterator(ctx)
	iter.SetNodes([]*structs.Node{
		regular[0], draining[0], draining[1], regular[1], draining[2],
	})

	// Power-of-two samples rotate through the draining partition, including
	// wrapping around, rather than restarting at its first two nodes.
	for _, sample := range [][]*structs.Node{
		{draining[0], draining[1]},
		{draining[2], draining[0]},
		{draining[1], draining[2]},
	} {
		iter.Reset()
		must.Eq(t, sample[0], iter.Next())
		must.Eq(t, sample[1], iter.Next())
	}

	// Each full scan still visits every node exactly once, with draining
	// candidates first and the original order preserved within each partition.
	iter.Reset()
	must.Eq(t, []*structs.Node{
		draining[0], draining[1], draining[2], regular[0], regular[1],
	}, collectFeasible(iter))
}
