// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"pgregory.net/rapid"
)

func TestNodeReconciler_PropTest(t *testing.T) {
	t.Run("system jobs", rapid.MakeCheck(func(t *rapid.T) {
		nr := genNodeReconciler(structs.JobTypeSystem, &idGenerator{}).Draw(t, "input")
		results := Node(nr.job, nr.readyNodes, nr.notReadyNodes,
			nr.taintedNodes, nr.allocs, nr.terminal, nr.serverSupportsDisconnectedClients)
		if results == nil {
			t.Fatal("results should never be nil")
		}
		// TODO(tgross): this where the properties under test go
	}))

	t.Run("sysbatch jobs", rapid.MakeCheck(func(t *rapid.T) {
		nr := genNodeReconciler(structs.JobTypeSysBatch, &idGenerator{}).Draw(t, "input")
		results := Node(nr.job, nr.readyNodes, nr.notReadyNodes,
			nr.taintedNodes, nr.allocs, nr.terminal, nr.serverSupportsDisconnectedClients)
		if results == nil {
			t.Fatal("results should never be nil")
		}
		// TODO(tgross): this where the properties under test go
	}))

}

type nodeReconcilerInput struct {
	job                               *structs.Job
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
		nodes := rapid.SliceOfN(genNode(idg), 1, 30).Draw(t, "nodes")
		job := genJob(jobType, idg).Draw(t, "job")
		taintedNodes := map[string]*structs.Node{}
		notReadyNodes := map[string]struct{}{}
		readyNodes := []*structs.Node{}
		terminal := structs.TerminalByNodeByName{}
		allocs := []*structs.Allocation{}

		for _, node := range nodes {
			alloc := genExistingAlloc(idg, job, node.ID, now).Draw(t, "existing_alloc")
			alloc.Name = job.ID + "." + alloc.TaskGroup + "[0]"
			if alloc.TerminalStatus() {
				terminal[node.ID] = map[string]*structs.Allocation{alloc.Name: alloc}
			}
			allocs = append(allocs, alloc)
			if node.Ready() {
				readyNodes = append(readyNodes, node)
			} else {
				// TODO(tgross): are these really different?
				notReadyNodes[node.ID] = struct{}{}
				taintedNodes[node.ID] = node
			}
		}

		return &nodeReconcilerInput{
			job:                               job,
			readyNodes:                        readyNodes,
			notReadyNodes:                     notReadyNodes,
			taintedNodes:                      taintedNodes,
			allocs:                            allocs,
			serverSupportsDisconnectedClients: rapid.Bool().Draw(t, "supports_disconnected"),
		}
	})
}
