// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package benchmarks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

// BenchmarkSchedulerExample is an example of how to write a one-off
// benchmark for the Nomad scheduler. The starting state for your
// implementation will depend on the following environment variables:
//
//   - NOMAD_BENCHMARK_DATADIR: path to data directory
//   - NOMAD_BENCHMARK_SNAPSHOT: path to raft snapshot
//   - neither: empty starting state
//
// You can run a profile for this benchmark with the usual -cpuprofile
// -memprofile flags.
func BenchmarkSchedulerExample(b *testing.B) {

	h := NewBenchmarkingHarness(b)
	var eval *structs.Evaluation

	// (implement me!) this is your setup for the state and the eval
	// you're going to process, all of which happens before benchmarking
	// starts. If you're benchmarking a real world datadir or snapshot,
	// you should assert your assumptions about the contents here.
	{
		upsertNodes(h, 5000, 100)

		iter, err := h.State.Nodes(nil)
		require.NoError(b, err)
		nodes := 0
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			nodes++
		}
		require.Equal(b, 5000, nodes)
		job := generateJob(true, 600)
		eval = upsertJob(h, job)
	}

	// (implement me!) Note that h.Process doesn't return errors for
	// most states that result in blocked plans, so it's recommended
	// you write an assertion section here so that you're sure you're
	// benchmarking a successful run and not a failed plan.
	{
		err := h.Process(scheduler.NewServiceScheduler, eval)
		require.NoError(b, err)
		require.Len(b, h.Plans, 1)
		require.False(b, h.Plans[0].IsNoOp())
	}

	for i := 0; i < b.N; i++ {
		err := h.Process(scheduler.NewServiceScheduler, eval)
		require.NoError(b, err)
	}
}

// BenchmarkServiceScheduler exercises the service scheduler at a
// variety of cluster sizes, with both spread and non-spread jobs
func BenchmarkServiceScheduler(b *testing.B) {

	clusterSizes := []int{1000, 5000, 10000}
	rackSets := []int{10, 25, 50, 75}
	jobSizes := []int{300, 600, 900, 1200}

	type benchmark struct {
		name        string
		clusterSize int
		racks       int
		jobSize     int
		withSpread  bool
	}

	benchmarks := []benchmark{}
	for _, clusterSize := range clusterSizes {
		for _, racks := range rackSets {
			for _, jobSize := range jobSizes {
				benchmarks = append(benchmarks,
					benchmark{
						name: fmt.Sprintf("%d nodes %d racks %d allocs spread",
							clusterSize, racks, jobSize,
						),
						clusterSize: clusterSize, racks: racks, jobSize: jobSize,
						withSpread: true,
					},
				)
				benchmarks = append(benchmarks,
					benchmark{
						name: fmt.Sprintf("%d nodes %d racks %d allocs no spread",
							clusterSize, racks, jobSize,
						),
						clusterSize: clusterSize, racks: racks, jobSize: jobSize,
						withSpread: false,
					},
				)
			}
		}
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			h := scheduler.NewHarness(b)
			upsertNodes(h, bm.clusterSize, bm.racks)
			job := generateJob(bm.withSpread, bm.jobSize)
			eval := upsertJob(h, job)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := h.Process(scheduler.NewServiceScheduler, eval)
				require.NoError(b, err)
			}
		})
	}
}

func upsertJob(h *scheduler.Harness, job *structs.Job) *structs.Evaluation {
	err := h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job)
	if err != nil {
		panic(err)
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
		panic(err)
	}
	return eval
}

func generateJob(withSpread bool, jobSize int) *structs.Job {
	job := mock.Job()
	job.Datacenters = []string{"dc-1", "dc-2"}
	if withSpread {
		job.Spreads = []*structs.Spread{{Attribute: "${meta.rack}"}}
	}
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

func upsertNodes(h *scheduler.Harness, count, racks int) {

	datacenters := []string{"dc-1", "dc-2"}

	for i := 0; i < count; i++ {
		node := mock.Node()
		node.Datacenter = datacenters[i%2]
		node.Meta = map[string]string{}
		node.Meta["rack"] = fmt.Sprintf("r%d", i%racks)
		cpuShares := 14000
		memoryMB := 32000
		diskMB := 100 * 1024

		node.NodeResources = &structs.NodeResources{
			Cpu: structs.NodeCpuResources{
				CpuShares: int64(cpuShares),
			},
			Memory: structs.NodeMemoryResources{
				MemoryMB: int64(memoryMB),
			},
			Disk: structs.NodeDiskResources{
				DiskMB: int64(diskMB),
			},
			Networks: []*structs.NetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		}

		err := h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node)
		if err != nil {
			panic(err)
		}
	}
}
