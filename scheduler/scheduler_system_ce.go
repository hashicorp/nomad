// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package scheduler

import (
	"slices"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/feasible"
	"github.com/hashicorp/nomad/scheduler/reconciler"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

// findNodesForTG runs feasibility checks on nodes. The result includes all nodes for each
// task group (feasible and infeasible) along with metrics information on the checks.
func (s *SystemScheduler) findNodesForTG(buckets *reconciler.NodeReconcileResult) (tgNodes map[string]taskGroupNodes, filteredMetrics map[string]*structs.AllocMetric) {
	tgNodes = make(map[string]taskGroupNodes)
	filteredMetrics = make(map[string]*structs.AllocMetric)

	nodeByID := make(map[string]*structs.Node, len(s.nodes))
	for _, node := range s.nodes {
		nodeByID[node.ID] = node
	}

	nodes := make([]*structs.Node, 1)
	for _, a := range slices.Concat(buckets.Place, buckets.Update, buckets.Ignore) {
		tgName := a.TaskGroup.Name
		if tgNodes[tgName] == nil {
			tgNodes[tgName] = taskGroupNodes{}
		}

		node, ok := nodeByID[a.Alloc.NodeID]
		if !ok {
			s.logger.Debug("could not find node", "node", a.Alloc.NodeID)
			continue
		}

		// Update the set of placement nodes
		nodes[0] = node
		s.stack.SetNodes(nodes)

		if a.Alloc.ID != "" {
			// temporarily include the old alloc from a destructive update so
			// that we can account for resources that will be freed by that
			// allocation. We'll back this change out if we end up needing to
			// limit placements by max_parallel or canaries.
			s.plan.AppendStoppedAlloc(a.Alloc, sstructs.StatusAllocUpdating, "", "")
		}

		// Attempt to match the task group
		option := s.stack.Select(a.TaskGroup, &feasible.SelectOptions{AllocName: a.Name})

		// Always store the results. Keep the metrics that were generated
		// for the match attempt so they can be used during placement.
		tgNodes[tgName] = append(tgNodes[tgName], &taskGroupNode{node.ID, option, s.ctx.Metrics().Copy()})

		if option == nil {
			// When no match is found, merge the filter metrics for the task
			// group so proper reporting can be done during placement.
			filteredMetrics[tgName] = mergeNodeFiltered(filteredMetrics[tgName], s.ctx.Metrics())
		}
	}
	return
}
