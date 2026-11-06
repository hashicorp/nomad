// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package feasible

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// DrainChecker is a transient check: neither drain state nor remaining time is
// a property of a computed node class.
type DrainChecker struct {
	ctx       Context
	job       *structs.Job
	tg        *structs.TaskGroup
	startTime time.Time
}

func NewDrainChecker(ctx Context) *DrainChecker {
	return &DrainChecker{ctx: ctx}
}

func (c *DrainChecker) SetJob(j *structs.Job) {
	c.job = j
}

func (c *DrainChecker) SetTaskGroup(tg *structs.TaskGroup, existing *structs.Allocation, startTime time.Time) {
	c.tg = tg
	if existing == nil {
		c.startTime = startTime
	} else {
		c.startTime = time.Unix(0, existing.CreateTime)
	}
}

func (c *DrainChecker) Feasible(node *structs.Node) bool {
	// Feasible if not draining
	if node.DrainStrategy == nil {
		return true
	}

	// Infeasible if draining and not duration-aware
	if !node.DrainStrategy.DurationAware {
		c.ctx.Metrics().FilterNode(node, "node drain not duration-aware")
		return false
	}

	feasible := node.CanBackfill(c.job, c.tg, c.startTime)
	if !feasible {
		c.ctx.Metrics().FilterNode(node, "node drain time budget")
	}
	return feasible
}

// DrainPriorityIterator visits duration-aware draining nodes before ordinary
// nodes so eligible backfill capacity is considered even with a small node
// sampling limit. All nodes still pass through the usual feasibility checks.
type DrainPriorityIterator struct {
	nodes    []*structs.Node
	draining *StaticIterator
	regular  *StaticIterator
}

func NewDrainPriorityIterator(ctx Context) *DrainPriorityIterator {
	return &DrainPriorityIterator{
		draining: NewStaticIterator(ctx, nil),
		regular:  NewStaticIterator(ctx, nil),
	}
}

func (iter *DrainPriorityIterator) SetNodes(nodes []*structs.Node) {
	iter.nodes = nodes
	drainingCount := 0
	for _, node := range nodes {
		if node.DrainStrategy != nil && node.DrainStrategy.DurationAware {
			drainingCount++
		}
	}
	if drainingCount == 0 {
		iter.draining.SetNodes(nil)
		iter.regular.SetNodes(nodes)
		return
	}

	draining := make([]*structs.Node, 0, drainingCount)
	regular := make([]*structs.Node, 0, len(nodes)-drainingCount)
	for _, node := range nodes {
		if node.DrainStrategy != nil && node.DrainStrategy.DurationAware {
			draining = append(draining, node)
		} else {
			regular = append(regular, node)
		}
	}
	iter.draining.SetNodes(draining)
	iter.regular.SetNodes(regular)
}

func (iter *DrainPriorityIterator) Next() *structs.Node {
	if node := iter.draining.Next(); node != nil {
		return node
	}
	return iter.regular.Next()
}

func (iter *DrainPriorityIterator) Reset() {
	// Preserve rotation within both shuffled partitions while starting each
	// placement with draining candidates.
	iter.draining.Reset()
	iter.regular.Reset()
}

// drainAffinityScore deliberately outweighs the normalized binpack score
// (0..1), even when averaged with other affinity and penalty scores. Backfill
// uses capacity that is about to disappear and should almost always win over
// a tighter fit on an ordinary node.
const drainAffinityScore = 10.0

// DrainAffinityIterator rewards placements that fit the drain time budget.
type DrainAffinityIterator struct {
	ctx    Context
	source RankIterator
	drain  *DrainChecker
}

func NewDrainAffinityIterator(ctx Context, source RankIterator, drain *DrainChecker) *DrainAffinityIterator {
	return &DrainAffinityIterator{ctx: ctx, source: source, drain: drain}
}

func (iter *DrainAffinityIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	if option.Node.CanBackfill(iter.drain.job, iter.drain.tg, iter.drain.startTime) {
		option.Scores = append(option.Scores, drainAffinityScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "drain-affinity", drainAffinityScore)
	}
	return option
}

func (iter *DrainAffinityIterator) Reset() {
	iter.source.Reset()
}
