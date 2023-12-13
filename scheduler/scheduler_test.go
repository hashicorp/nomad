// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/iterator"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestScheduler_JobRegister_MemoryMaxHonored(t *testing.T) {
	ci.Parallel(t)

	// Test node pools.
	poolNoSchedConfig := mock.NodePool()
	poolNoSchedConfig.SchedulerConfiguration = nil

	poolWithMemOversub := mock.NodePool()
	poolWithMemOversub.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		MemoryOversubscriptionEnabled: pointer.Of(true),
	}

	poolNoMemOversub := mock.NodePool()
	poolNoMemOversub.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		MemoryOversubscriptionEnabled: pointer.Of(false),
	}

	cases := []struct {
		name                          string
		nodePool                      string
		cpu                           int
		memory                        int
		memoryMax                     int
		memoryOversubscriptionEnabled bool

		expectedTaskMemoryMax int
		// expectedTotalMemoryMax should be SUM(MAX(memory, memoryMax)) for all
		// tasks if memory oversubscription is enabled and SUM(memory) if it's
		// disabled.
		expectedTotalMemoryMax int
	}{
		{
			name:                          "plain no max",
			nodePool:                      poolNoSchedConfig.Name,
			cpu:                           100,
			memory:                        200,
			memoryMax:                     0,
			memoryOversubscriptionEnabled: true,

			expectedTaskMemoryMax:  0,
			expectedTotalMemoryMax: 200,
		},
		{
			name:                          "with max",
			nodePool:                      poolNoSchedConfig.Name,
			cpu:                           100,
			memory:                        200,
			memoryMax:                     300,
			memoryOversubscriptionEnabled: true,

			expectedTaskMemoryMax:  300,
			expectedTotalMemoryMax: 300,
		},
		{
			name:      "with max but disabled",
			nodePool:  poolNoSchedConfig.Name,
			cpu:       100,
			memory:    200,
			memoryMax: 300,

			memoryOversubscriptionEnabled: false,
			expectedTaskMemoryMax:         0,
			expectedTotalMemoryMax:        200, // same as no max
		},
		{
			name:                          "with max and enabled by node pool",
			nodePool:                      poolWithMemOversub.Name,
			cpu:                           100,
			memory:                        200,
			memoryMax:                     300,
			memoryOversubscriptionEnabled: false,

			expectedTaskMemoryMax:  300,
			expectedTotalMemoryMax: 300,
		},
		{
			name:                          "with max but disabled by node pool",
			nodePool:                      poolNoMemOversub.Name,
			cpu:                           100,
			memory:                        200,
			memoryMax:                     300,
			memoryOversubscriptionEnabled: true,

			expectedTaskMemoryMax:  0,
			expectedTotalMemoryMax: 200, // same as no max
		},
	}

	jobTypes := []string{
		"batch",
		"service",
		"sysbatch",
		"system",
	}

	for _, jobType := range jobTypes {
		for _, c := range cases {
			t.Run(fmt.Sprintf("%s/%s", jobType, c.name), func(t *testing.T) {
				h := NewHarness(t)

				// Create node pools.
				nodePools := []*structs.NodePool{
					poolNoSchedConfig,
					poolWithMemOversub,
					poolNoMemOversub,
				}
				h.State.UpsertNodePools(structs.MsgTypeTestSetup, h.NextIndex(), nodePools)

				// Create some nodes.
				for i := 0; i < 3; i++ {
					node := mock.Node()
					node.NodePool = c.nodePool
					must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
				}

				// Set global scheduler configuration.
				h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
					MemoryOversubscriptionEnabled: c.memoryOversubscriptionEnabled,
				})

				// Create test job.
				var job *structs.Job
				switch jobType {
				case "batch":
					job = mock.BatchJob()
				case "service":
					job = mock.Job()
				case "sysbatch":
					job = mock.SystemBatchJob()
				case "system":
					job = mock.SystemJob()
				}
				job.TaskGroups[0].Count = 1
				job.NodePool = c.nodePool

				task := job.TaskGroups[0].Tasks[0].Name
				res := job.TaskGroups[0].Tasks[0].Resources
				res.CPU = c.cpu
				res.MemoryMB = c.memory
				res.MemoryMaxMB = c.memoryMax

				must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

				// Create a mock evaluation to register the job
				eval := &structs.Evaluation{
					Namespace:   structs.DefaultNamespace,
					ID:          uuid.Generate(),
					Priority:    job.Priority,
					TriggeredBy: structs.EvalTriggerJobRegister,
					JobID:       job.ID,
					Status:      structs.EvalStatusPending,
				}

				must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

				// Process the evaluation
				var scheduler Factory
				switch jobType {
				case "batch":
					scheduler = NewBatchScheduler
				case "service":
					scheduler = NewServiceScheduler
				case "sysbatch":
					scheduler = NewSysBatchScheduler
				case "system":
					scheduler = NewSystemScheduler
				}
				err := h.Process(scheduler, eval)
				must.NoError(t, err)

				must.Len(t, 1, h.Plans)

				allocs, err := h.State.AllocsByJob(nil, job.Namespace, job.ID, false)
				must.NoError(t, err)

				// Ensure all allocations placed
				var expectedAllocCount int
				switch jobType {
				case "batch", "service":
					expectedAllocCount = 1
				case "system", "sysbatch":
					nodes, err := h.State.NodesByNodePool(nil, job.NodePool)
					must.NoError(t, err)
					expectedAllocCount = iterator.Len(nodes)
				}
				must.Len(t, expectedAllocCount, allocs)
				alloc := allocs[0]

				// checking new resources field deprecated Resources fields
				must.Eq(t, int64(c.cpu), alloc.AllocatedResources.Tasks[task].Cpu.CpuShares)
				must.Eq(t, int64(c.memory), alloc.AllocatedResources.Tasks[task].Memory.MemoryMB)
				must.Eq(t, int64(c.expectedTaskMemoryMax), alloc.AllocatedResources.Tasks[task].Memory.MemoryMaxMB)

				// checking old deprecated Resources fields
				must.Eq(t, c.cpu, alloc.TaskResources[task].CPU)
				must.Eq(t, c.memory, alloc.TaskResources[task].MemoryMB)
				must.Eq(t, c.expectedTaskMemoryMax, alloc.TaskResources[task].MemoryMaxMB)

				// check total resource fields - alloc.Resources deprecated field, no modern equivalent
				must.Eq(t, c.cpu, alloc.Resources.CPU)
				must.Eq(t, c.memory, alloc.Resources.MemoryMB)
				must.Eq(t, c.expectedTotalMemoryMax, alloc.Resources.MemoryMaxMB)
			})
		}
	}
}
