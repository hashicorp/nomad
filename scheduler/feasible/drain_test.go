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

// Drain feasibility must apply independently of the resource scoring algorithm.
func TestGenericStack_DrainFeasibility(t *testing.T) {
	ci.Parallel(t)

	for _, algorithm := range []structs.SchedulerAlgorithm{
		structs.SchedulerAlgorithmBinpack,
		structs.SchedulerAlgorithmSpread,
	} {
		t.Run(string(algorithm), func(t *testing.T) {
			_, ctx := MockContext(t)
			job := mock.BatchJob()
			tg := job.TaskGroups[0]
			tg.MaxRunDuration = new(time.Minute)
			node := mock.Node()
			node.DrainStrategy = &structs.DrainStrategy{
				DrainSpec:     structs.DrainSpec{DurationAware: true},
				ForceDeadline: time.Now().Add(10 * time.Minute),
			}
			stack := NewGenericStack(true, ctx)
			stack.SetSchedulerConfiguration(&structs.SchedulerConfiguration{
				SchedulerAlgorithm: algorithm,
			})
			stack.SetNodes([]*structs.Node{node})
			stack.SetJob(job)

			option := stack.Select(tg, &SelectOptions{})
			must.NotNil(t, option)
			must.Eq(t, node.ID, option.Node.ID)
			must.Zero(t, ctx.Metrics().NodesFiltered)

			// Recheck the same node after its time budget no longer fits;
			// drain feasibility must not be cached by node class.
			node.DrainStrategy.ForceDeadline = time.Now().Add(30 * time.Second)
			must.Nil(t, stack.Select(tg, &SelectOptions{}))
			must.Eq(t, map[string]int{"node drain time budget": 1},
				ctx.Metrics().ConstraintFiltered)
		})
	}
}
