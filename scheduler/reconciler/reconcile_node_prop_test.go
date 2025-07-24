// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"pgregory.net/rapid"
)

func TestNodeReconciler_PropTest(t *testing.T) {

	// collectExpectedAndResults returns a convenience map that may hold
	// multiple "states" for the same alloc (ex. all three of "total" and
	// "terminal" and "failed")
	collectExpectedAndResults := func(nr *nodeReconcilerInput, results *NodeReconcileResult) map[string]map[string]int {
		perTaskGroup := map[string]map[string]int{}
		for _, tg := range nr.job.TaskGroups {
			perTaskGroup[tg.Name] = map[string]int{"expect_count": tg.Count}
		}

		for _, alloc := range nr.allocs {
			if _, ok := perTaskGroup[alloc.TaskGroup]; !ok {
				// existing task group doesn't exist in new job
				perTaskGroup[alloc.TaskGroup] = map[string]int{"expect_count": 0}
			}
			perTaskGroup[alloc.TaskGroup]["exist_total"]++
			perTaskGroup[alloc.TaskGroup]["exist_"+alloc.ClientStatus]++
			if alloc.TerminalStatus() {
				perTaskGroup[alloc.TaskGroup]["exist_terminal"]++
			}
		}

		// NodeReconcileResults doesn't split results by task group, so split
		// them up so we can check them separately
		recordResult := func(subresult []AllocTuple, label string) {
			if subresult != nil {
				for _, alloc := range subresult {
					var tgName string
					if alloc.TaskGroup != nil {
						tgName = alloc.TaskGroup.Name
					} else if alloc.Alloc != nil {
						tgName = alloc.Alloc.TaskGroup
					} else {
						t.Fatal("one of task group or alloc must always be non-nil")
					}
					perTaskGroup[tgName][label]++
				}
			}
		}

		recordResult(results.Place, "placed")
		recordResult(results.Ignore, "ignore")
		recordResult(results.Disconnecting, "disconnecting")
		recordResult(results.Lost, "lost")
		recordResult(results.Migrate, "migrate")
		recordResult(results.Reconnecting, "reconnecting")
		recordResult(results.Stop, "stop")
		recordResult(results.Update, "update")

		return perTaskGroup
	}

	// sharedSafetyProperties asserts safety properties ("something bad never
	// happens") that apply to all job types that use the node reconciler
	sharedSafetyProperties := func(t *rapid.T, nr *nodeReconcilerInput, results *NodeReconcileResult, perTaskGroup map[string]map[string]int) {
		t.Helper()

		if !nr.serverSupportsDisconnectedClients {
			must.Len(t, 0, results.Disconnecting,
				must.Sprint("groups that don't support disconnected clients should never result in disconnecting"))
			must.Len(t, 0, results.Reconnecting,
				must.Sprint("groups that don't support disconnected clients should never result in reconnecting"))
		}

		for tgName, counts := range perTaskGroup {
			must.LessEq(t, counts["expect_count"]*len(nr.readyNodes), counts["place"],
				must.Sprintf("group placements should never exceed ready nodes times count (%s): %v",
					tgName, counts))

			must.LessEq(t, counts["exist_total"], counts["migrate"],
				must.Sprintf("group migrate should never exceed total allocs in group (%s): %v",
					tgName, counts))
			must.LessEq(t, counts["exist_total"], counts["ignore"],
				must.Sprintf("group ignores should never exceed total allocs in group (%s): %v",
					tgName, counts))
			must.LessEq(t, counts["exist_total"], counts["stop"],
				must.Sprintf("group stops should never exceed total allocs in group (%s): %v",
					tgName, counts))
			must.LessEq(t, counts["exist_total"], counts["update"],
				must.Sprintf("group updates should never exceed total allocs in group (%s): %v",
					tgName, counts))

		}
	}

	t.Run("system jobs", rapid.MakeCheck(func(t *rapid.T) {
		nr := genNodeReconciler(structs.JobTypeSystem, &idGenerator{}).Draw(t, "input")
		results := Node(nr.job, nr.evalPriority, nr.deployment, nr.readyNodes,
			nr.notReadyNodes, nr.taintedNodes, nr.allocs, nr.terminal,
			nr.serverSupportsDisconnectedClients)
		must.NotNil(t, results, must.Sprint("results should never be nil"))
		perTaskGroup := collectExpectedAndResults(nr, results)

		sharedSafetyProperties(t, nr, results, perTaskGroup)
	}))

	t.Run("sysbatch jobs", rapid.MakeCheck(func(t *rapid.T) {
		nr := genNodeReconciler(structs.JobTypeSysBatch, &idGenerator{}).Draw(t, "input")
		results := Node(nr.job, nr.evalPriority, nr.deployment, nr.readyNodes,
			nr.notReadyNodes, nr.taintedNodes, nr.allocs, nr.terminal,
			nr.serverSupportsDisconnectedClients)
		must.NotNil(t, results, must.Sprint("results should never be nil"))
		perTaskGroup := collectExpectedAndResults(nr, results)

		sharedSafetyProperties(t, nr, results, perTaskGroup)
	}))

}

type nodeReconcilerInput struct {
	job                               *structs.Job
	evalPriority                      int
	deployment                        *structs.Deployment
	readyNodes                        []*structs.Node
	notReadyNodes                     map[string]struct{}
	taintedNodes                      map[string]*structs.Node
	allocs                            []*structs.Allocation
	terminal                          structs.TerminalByNodeByName
	serverSupportsDisconnectedClients bool
}

func genNodeReconciler(jobType string, idg *idGenerator) *rapid.Generator[*nodeReconcilerInput] {
	return rapid.Custom(func(t *rapid.T) *nodeReconcilerInput {
		now := time.Now() // note: you can only use offsets from this
		nodes := rapid.SliceOfN(genNode(idg), 0, 30).Draw(t, "nodes")
		empty := rapid.SliceOfN(genNode(idg), 0, 5).Draw(t, "empty_nodes")

		job := genJob(jobType, idg).Draw(t, "job")
		oldJob := job.Copy()
		oldJob.Version--
		oldJob.JobModifyIndex = 100
		oldJob.CreateIndex = 100

		taintedNodes := map[string]*structs.Node{}
		notReadyNodes := map[string]struct{}{}
		readyNodes := []*structs.Node{}
		terminal := structs.TerminalByNodeByName{}
		live := []*structs.Allocation{}

		for _, node := range nodes {
			j := job
			isOld := weightedBool(30).Draw(t, "is_old")
			if isOld {
				j = oldJob
			}

			alloc := genExistingAlloc(idg, j, node.ID, now).Draw(t, "existing_alloc")
			alloc.Name = job.ID + "." + alloc.TaskGroup + "[0]"
			if alloc.TerminalStatus() {
				terminal[node.ID] = map[string]*structs.Allocation{alloc.Name: alloc}
			} else {
				live = append(live, alloc)
			}

			if isOld && weightedBool(20).Draw(t, "wrong_dc") {
				// put some of the old allocs on nodes we'll no longer consider
				notReadyNodes[node.ID] = struct{}{}
			} else if node.Ready() {
				readyNodes = append(readyNodes, node)
			} else {
				notReadyNodes[node.ID] = struct{}{}
				if structs.ShouldDrainNode(node.Status) || node.DrainStrategy != nil {
					taintedNodes[node.ID] = node
					alloc.DesiredTransition = structs.DesiredTransition{
						Migrate: pointer.Of(true),
					}
				}
				if node.Status == structs.NodeStatusDisconnected {
					taintedNodes[node.ID] = node
				}
			}
		}
		for _, node := range empty {
			if node.Ready() {
				readyNodes = append(readyNodes, node)
			} else {
				notReadyNodes[node.ID] = struct{}{}
			}
		}

		deployment := genDeployment(idg, job, live).Draw(t, "deployment")

		return &nodeReconcilerInput{
			job:                               job,
			evalPriority:                      rapid.Int().Draw(t, "eval_priority"),
			deployment:                        deployment,
			readyNodes:                        readyNodes,
			notReadyNodes:                     notReadyNodes,
			taintedNodes:                      taintedNodes,
			allocs:                            live,
			serverSupportsDisconnectedClients: rapid.Bool().Draw(t, "supports_disconnected"),
		}
	})
}
