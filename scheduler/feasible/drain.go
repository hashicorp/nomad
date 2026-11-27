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
