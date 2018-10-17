package scheduler

import (
	"testing"

	"fmt"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestResourceDistance(t *testing.T) {
	resourceAsk := &structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Cpu: structs.AllocatedCpuResources{
				CpuShares: 2048,
			},
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: 512,
			},
			Networks: []*structs.NetworkResource{
				{
					Device: "eth0",
					MBits:  1024,
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 4096,
		},
	}

	type testCase struct {
		allocResource    *structs.ComparableResources
		expectedDistance string
	}

	testCases := []*testCase{
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 512,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 4096,
				},
			},
			"0.000",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 400,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 1024,
				},
			},
			"0.928",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 8192,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 200,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  512,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 1024,
				},
			},
			"3.152",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 500,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 4096,
				},
			},
			"0.023",
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			require := require.New(t)
			actualDistance := fmt.Sprintf("%3.3f", basicResourceDistance(resourceAsk, tc.allocResource))
			require.Equal(tc.expectedDistance, actualDistance)
		})

	}

}

func TestPreemption(t *testing.T) {
	type testCase struct {
		desc                 string
		currentAllocations   []*structs.Allocation
		nodeReservedCapacity *structs.Resources
		nodeCapacity         *structs.Resources
		resourceAsk          *structs.Resources
		jobPriority          int
		currentPreemptions   []*structs.Allocation
		preemptedAllocIDs    map[string]struct{}
	}

	highPrioJob := mock.Job()
	highPrioJob.Priority = 100

	lowPrioJob := mock.Job()
	lowPrioJob.Priority = 30

	lowPrioJob2 := mock.Job()
	lowPrioJob2.Priority = 30

	// Create some persistent alloc ids to use in test cases
	allocIDs := []string{uuid.Generate(), uuid.Generate(), uuid.Generate(), uuid.Generate(), uuid.Generate()}

	nodeResources := &structs.Resources{
		CPU:      4000,
		MemoryMB: 8192,
		DiskMB:   100 * 1024,
		IOPS:     150,
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
	}
	reservedNodeResources := &structs.Resources{
		CPU:      100,
		MemoryMB: 256,
		DiskMB:   4 * 1024,
		Networks: []*structs.NetworkResource{
			{
				Device:        "eth0",
				IP:            "192.168.0.100",
				ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
				MBits:         1,
			},
		},
	}

	testCases := []testCase{
		{
			desc: "No preemption because existing allocs are not low priority",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      3200,
					MemoryMB: 7256,
					DiskMB:   4 * 1024,
					IOPS:     150,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  50,
						},
					},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      2000,
				MemoryMB: 256,
				DiskMB:   4 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
						MBits:         1,
					},
				},
			},
		},
		{
			desc: "Preempting low priority allocs not enough to meet resource ask",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      3200,
					MemoryMB: 7256,
					DiskMB:   4 * 1024,
					IOPS:     150,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  50,
						},
					},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      4000,
				MemoryMB: 8192,
				DiskMB:   4 * 1024,
				IOPS:     300,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
						MBits:         1,
					},
				},
			},
		},
		{
			desc: "Combination of high/low priority allocs, without static ports",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					IOPS:     150,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  500,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
				createAlloc(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      700,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1100,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  840,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		}, {
			desc: "Preemption needed for all resources except network",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   40 * 1024,
					IOPS:     100,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  50,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   25 * 1024,
				}),
				createAlloc(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      700,
					MemoryMB: 276,
					DiskMB:   20 * 1024,
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 3000,
				DiskMB:   50 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  50,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		},
		{
			desc: "Only one low priority alloc needs to be preempted",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  500,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  320,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      300,
				MemoryMB: 500,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  320,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[2]: {},
			},
		},
		{
			desc: "one alloc meets static port need, another meets remaining mbits needed",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					IOPS:     150,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  500,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  200,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      2700,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  800,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
			},
		},
		{
			desc: "alloc that meets static port need also meets other needds",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  600,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  100,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      600,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  700,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
		{
			desc: "alloc from job that has existing evictions not chosen for preemption",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     10,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  500,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob2, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     10,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      300,
				MemoryMB: 500,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  320,
					},
				},
			},
			currentPreemptions: []*structs.Allocation{
				createAlloc(allocIDs[4], lowPrioJob2, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					IOPS:     10,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
		// This test case exercises the code path for a final filtering step that tries to
		// minimize the number of preemptible allocations
		{
			desc: "Filter out allocs whose resource usage superset is also in the preemption list",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					IOPS:     50,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      1500,
					MemoryMB: 256,
					DiskMB:   5 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  100,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      600,
					MemoryMB: 256,
					DiskMB:   5 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  300,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         nodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 256,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  50,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			node := mock.Node()
			node.Resources = tc.nodeCapacity
			node.Reserved = tc.nodeReservedCapacity

			state, ctx := testContext(t)
			nodes := []*RankedNode{
				{
					Node: node,
				},
			}
			state.UpsertNode(1000, node)
			for _, alloc := range tc.currentAllocations {
				alloc.NodeID = node.ID
			}
			require := require.New(t)
			err := state.UpsertAllocs(1001, tc.currentAllocations)

			require.Nil(err)
			if tc.currentPreemptions != nil {
				ctx.plan.NodePreemptions[node.ID] = tc.currentPreemptions
			}
			static := NewStaticRankIterator(ctx, nodes)
			binPackIter := NewBinPackIterator(ctx, static, true, tc.jobPriority)

			taskGroup := &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name:      "web",
						Resources: tc.resourceAsk,
					},
				},
			}

			binPackIter.SetTaskGroup(taskGroup)
			option := binPackIter.Next()
			if tc.preemptedAllocIDs == nil {
				require.Nil(option)
			} else {
				require.NotNil(option)
				preemptedAllocs := option.PreemptedAllocs
				require.Equal(len(tc.preemptedAllocIDs), len(preemptedAllocs))
				for _, alloc := range preemptedAllocs {
					_, ok := tc.preemptedAllocIDs[alloc.ID]
					require.True(ok)
				}
			}
		})
	}
}

// helper method to create allocations with given jobs and resources
func createAlloc(id string, job *structs.Job, resource *structs.Resources) *structs.Allocation {
	alloc := &structs.Allocation{
		ID:    id,
		Job:   job,
		JobID: job.ID,
		TaskResources: map[string]*structs.Resources{
			"web": resource,
		},
		Resources:     resource,
		Namespace:     structs.DefaultNamespace,
		EvalID:        uuid.Generate(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusRunning,
		TaskGroup:     "web",
	}
	return alloc
}
