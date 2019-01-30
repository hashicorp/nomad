package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	require "github.com/stretchr/testify/require"
)

func TestFeasibleRankIterator(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	feasible := NewFeasibleRankIterator(ctx, static)

	out := collectRanked(feasible)
	if len(out) != len(nodes) {
		t.Fatalf("bad: %v", out)
	}
}

func TestBinPackIterator_NoExistingAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				// Perfect fit
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
				ReservedResources: &structs.NodeReservedResources{
					Cpu: structs.NodeReservedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.NodeReservedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// Overloaded
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 1024,
					},
				},
				ReservedResources: &structs.NodeReservedResources{
					Cpu: structs.NodeReservedCpuResources{
						CpuShares: 512,
					},
					Memory: structs.NodeReservedMemoryResources{
						MemoryMB: 512,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// 50% fit
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 4096,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 4096,
					},
				},
				ReservedResources: &structs.NodeReservedResources{
					Cpu: structs.NodeReservedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.NodeReservedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	taskGroup := &structs.TaskGroup{
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			{
				Name: "web",
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
	}
	binp := NewBinPackIterator(ctx, static, false, 0)
	binp.SetTaskGroup(taskGroup)

	scoreNorm := NewScoreNormalizationIterator(ctx, binp)

	out := collectRanked(scoreNorm)
	if len(out) != 2 {
		t.Fatalf("Bad: %v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[2] {
		t.Fatalf("Bad: %v", out)
	}

	if out[0].FinalScore != 1.0 {
		t.Fatalf("Bad Score: %v", out[0].FinalScore)
	}
	if out[1].FinalScore < 0.75 || out[1].FinalScore > 0.95 {
		t.Fatalf("Bad Score: %v", out[1].FinalScore)
	}
}

func TestBinPackIterator_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		{
			AllocatedResources: &structs.AllocatedResources{
				Tasks: map[string]*structs.AllocatedTaskResources{
					"web": {
						Cpu: structs.AllocatedCpuResources{
							CpuShares: 2048,
						},
						Memory: structs.AllocatedMemoryResources{
							MemoryMB: 2048,
						},
					},
				},
			},
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		{
			AllocatedResources: &structs.AllocatedResources{
				Tasks: map[string]*structs.AllocatedTaskResources{
					"web": {
						Cpu: structs.AllocatedCpuResources{
							CpuShares: 1024,
						},
						Memory: structs.AllocatedMemoryResources{
							MemoryMB: 1024,
						},
					},
				},
			},
		},
	}

	taskGroup := &structs.TaskGroup{
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			{
				Name: "web",
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
	}

	binp := NewBinPackIterator(ctx, static, false, 0)
	binp.SetTaskGroup(taskGroup)

	scoreNorm := NewScoreNormalizationIterator(ctx, binp)

	out := collectRanked(scoreNorm)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad Score: %v", out)
	}

	if out[0].FinalScore != 1.0 {
		t.Fatalf("Bad Score: %v", out[0].FinalScore)
	}
}

func TestBinPackIterator_ExistingAlloc(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    nodes[1].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	noErr(t, state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID)))
	noErr(t, state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	noErr(t, state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

	taskGroup := &structs.TaskGroup{
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			{
				Name: "web",
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
	}
	binp := NewBinPackIterator(ctx, static, false, 0)
	binp.SetTaskGroup(taskGroup)

	scoreNorm := NewScoreNormalizationIterator(ctx, binp)

	out := collectRanked(scoreNorm)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].FinalScore != 1.0 {
		t.Fatalf("Bad Score: %v", out[0].FinalScore)
	}
}

func TestBinPackIterator_ExistingAlloc_PlannedEvict(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    nodes[1].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	noErr(t, state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID)))
	noErr(t, state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	noErr(t, state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

	// Add a planned eviction to alloc1
	plan := ctx.Plan()
	plan.NodeUpdate[nodes[0].Node.ID] = []*structs.Allocation{alloc1}

	taskGroup := &structs.TaskGroup{
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			{
				Name: "web",
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
	}

	binp := NewBinPackIterator(ctx, static, false, 0)
	binp.SetTaskGroup(taskGroup)

	scoreNorm := NewScoreNormalizationIterator(ctx, binp)

	out := collectRanked(scoreNorm)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].FinalScore < 0.50 || out[0].FinalScore > 0.95 {
		t.Fatalf("Bad Score: %v", out[0].FinalScore)
	}
	if out[1].FinalScore != 1 {
		t.Fatalf("Bad Score: %v", out[1].FinalScore)
	}
}

// This is a fairly high level test that asserts the bin packer uses the device
// allocator properly. It is not intended to handle every possible device
// request versus availability scenario. That should be covered in device
// allocator tests.
func TestBinPackIterator_Devices(t *testing.T) {
	nvidiaNode := mock.NvidiaNode()
	devs := nvidiaNode.NodeResources.Devices[0].Instances
	nvidiaDevices := []string{devs[0].ID, devs[1].ID}

	nvidiaDev0 := mock.Alloc()
	nvidiaDev0.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{
		{
			Type:      "gpu",
			Vendor:    "nvidia",
			Name:      "1080ti",
			DeviceIDs: []string{nvidiaDevices[0]},
		},
	}

	type devPlacementTuple struct {
		Count      int
		ExcludeIDs []string
	}

	cases := []struct {
		Name               string
		Node               *structs.Node
		PlannedAllocs      []*structs.Allocation
		ExistingAllocs     []*structs.Allocation
		TaskGroup          *structs.TaskGroup
		NoPlace            bool
		ExpectedPlacements map[string]map[structs.DeviceIdTuple]devPlacementTuple
		DeviceScore        float64
	}{
		{
			Name: "single request, match",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 1,
								},
							},
						},
					},
				},
			},
			ExpectedPlacements: map[string]map[structs.DeviceIdTuple]devPlacementTuple{
				"web": {
					{
						Vendor: "nvidia",
						Type:   "gpu",
						Name:   "1080ti",
					}: {
						Count: 1,
					},
				},
			},
		},
		{
			Name: "single request multiple count, match",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 2,
								},
							},
						},
					},
				},
			},
			ExpectedPlacements: map[string]map[structs.DeviceIdTuple]devPlacementTuple{
				"web": {
					{
						Vendor: "nvidia",
						Type:   "gpu",
						Name:   "1080ti",
					}: {
						Count: 2,
					},
				},
			},
		},
		{
			Name: "single request, with affinities",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 1,
									Affinities: []*structs.Affinity{
										{
											LTarget: "${device.attr.graphics_clock}",
											Operand: ">",
											RTarget: "1.4 GHz",
											Weight:  90,
										},
									},
								},
							},
						},
					},
				},
			},
			ExpectedPlacements: map[string]map[structs.DeviceIdTuple]devPlacementTuple{
				"web": {
					{
						Vendor: "nvidia",
						Type:   "gpu",
						Name:   "1080ti",
					}: {
						Count: 1,
					},
				},
			},
			DeviceScore: 1.0,
		},
		{
			Name: "single request over count, no match",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 6,
								},
							},
						},
					},
				},
			},
			NoPlace: true,
		},
		{
			Name: "single request no device of matching type",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "fpga",
									Count: 1,
								},
							},
						},
					},
				},
			},
			NoPlace: true,
		},
		{
			Name: "single request with previous uses",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 1,
								},
							},
						},
					},
				},
			},
			ExpectedPlacements: map[string]map[structs.DeviceIdTuple]devPlacementTuple{
				"web": {
					{
						Vendor: "nvidia",
						Type:   "gpu",
						Name:   "1080ti",
					}: {
						Count:      1,
						ExcludeIDs: []string{nvidiaDevices[0]},
					},
				},
			},
			ExistingAllocs: []*structs.Allocation{nvidiaDev0},
		},
		{
			Name: "single request with planned uses",
			Node: nvidiaNode,
			TaskGroup: &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name: "web",
						Resources: &structs.Resources{
							CPU:      1024,
							MemoryMB: 1024,
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 1,
								},
							},
						},
					},
				},
			},
			ExpectedPlacements: map[string]map[structs.DeviceIdTuple]devPlacementTuple{
				"web": {
					{
						Vendor: "nvidia",
						Type:   "gpu",
						Name:   "1080ti",
					}: {
						Count:      1,
						ExcludeIDs: []string{nvidiaDevices[0]},
					},
				},
			},
			PlannedAllocs: []*structs.Allocation{nvidiaDev0},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			require := require.New(t)

			// Setup the context
			state, ctx := testContext(t)

			// Add the planned allocs
			if len(c.PlannedAllocs) != 0 {
				for _, alloc := range c.PlannedAllocs {
					alloc.NodeID = c.Node.ID
				}
				plan := ctx.Plan()
				plan.NodeAllocation[c.Node.ID] = c.PlannedAllocs
			}

			// Add the existing allocs
			if len(c.ExistingAllocs) != 0 {
				for _, alloc := range c.ExistingAllocs {
					alloc.NodeID = c.Node.ID
				}
				require.NoError(state.UpsertAllocs(1000, c.ExistingAllocs))
			}

			static := NewStaticRankIterator(ctx, []*RankedNode{{Node: c.Node}})
			binp := NewBinPackIterator(ctx, static, false, 0)
			binp.SetTaskGroup(c.TaskGroup)

			out := binp.Next()
			if out == nil && !c.NoPlace {
				t.Fatalf("expected placement")
			}

			// Check we got the placements we are expecting
			for tname, devices := range c.ExpectedPlacements {
				tr, ok := out.TaskResources[tname]
				require.True(ok)

				want := len(devices)
				got := 0
				for _, placed := range tr.Devices {
					got++

					expected, ok := devices[*placed.ID()]
					require.True(ok)
					require.Equal(expected.Count, len(placed.DeviceIDs))
					for _, id := range expected.ExcludeIDs {
						require.NotContains(placed.DeviceIDs, id)
					}
				}

				require.Equal(want, got)
			}

			// Check potential affinity scores
			if c.DeviceScore != 0.0 {
				require.Len(out.Scores, 2)
				require.Equal(c.DeviceScore, out.Scores[1])
			}
		})
	}
}

func TestJobAntiAffinity_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				ID: uuid.Generate(),
			},
		},
		{
			Node: &structs.Node{
				ID: uuid.Generate(),
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	job.ID = "foo"
	tg := job.TaskGroups[0]
	tg.Count = 4

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		{
			ID:        uuid.Generate(),
			JobID:     "foo",
			TaskGroup: tg.Name,
		},
		{
			ID:        uuid.Generate(),
			JobID:     "foo",
			TaskGroup: tg.Name,
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		{
			JobID: "bar",
		},
	}

	jobAntiAff := NewJobAntiAffinityIterator(ctx, static, "foo")
	jobAntiAff.SetJob(job)
	jobAntiAff.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, jobAntiAff)

	out := collectRanked(scoreNorm)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] {
		t.Fatalf("Bad: %v", out)
	}
	// Score should be -(#collissions+1/desired_count) => -(3/4)
	if out[0].FinalScore != -0.75 {
		t.Fatalf("Bad Score: %#v", out[0].FinalScore)
	}

	if out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[1].FinalScore != 0.0 {
		t.Fatalf("Bad Score: %v", out[1].FinalScore)
	}
}

func collectRanked(iter RankIterator) (out []*RankedNode) {
	for {
		next := iter.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}
	return
}

func TestNodeAntiAffinity_PenaltyNodes(t *testing.T) {
	_, ctx := testContext(t)
	node1 := &structs.Node{
		ID: uuid.Generate(),
	}
	node2 := &structs.Node{
		ID: uuid.Generate(),
	}

	nodes := []*RankedNode{
		{
			Node: node1,
		},
		{
			Node: node2,
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	nodeAntiAffIter := NewNodeReschedulingPenaltyIterator(ctx, static)
	nodeAntiAffIter.SetPenaltyNodes(map[string]struct{}{node1.ID: {}})

	scoreNorm := NewScoreNormalizationIterator(ctx, nodeAntiAffIter)

	out := collectRanked(scoreNorm)

	require := require.New(t)
	require.Equal(2, len(out))
	require.Equal(node1.ID, out[0].Node.ID)
	require.Equal(-1.0, out[0].FinalScore)

	require.Equal(node2.ID, out[1].Node.ID)
	require.Equal(0.0, out[1].FinalScore)

}

func TestScoreNormalizationIterator(t *testing.T) {
	// Test normalized scores when there is more than one scorer
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				ID: uuid.Generate(),
			},
		},
		{
			Node: &structs.Node{
				ID: uuid.Generate(),
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	job.ID = "foo"
	tg := job.TaskGroups[0]
	tg.Count = 4

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		{
			ID:        uuid.Generate(),
			JobID:     "foo",
			TaskGroup: tg.Name,
		},
		{
			ID:        uuid.Generate(),
			JobID:     "foo",
			TaskGroup: tg.Name,
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		{
			JobID: "bar",
		},
	}

	jobAntiAff := NewJobAntiAffinityIterator(ctx, static, "foo")
	jobAntiAff.SetJob(job)
	jobAntiAff.SetTaskGroup(tg)

	nodeReschedulePenaltyIter := NewNodeReschedulingPenaltyIterator(ctx, jobAntiAff)
	nodeReschedulePenaltyIter.SetPenaltyNodes(map[string]struct{}{nodes[0].Node.ID: {}})

	scoreNorm := NewScoreNormalizationIterator(ctx, nodeReschedulePenaltyIter)

	out := collectRanked(scoreNorm)
	require := require.New(t)

	require.Equal(2, len(out))
	require.Equal(out[0], nodes[0])
	// Score should be averaged between both scorers
	// -0.75 from job anti affinity and -1 from node rescheduling penalty
	require.Equal(-0.875, out[0].FinalScore)
	require.Equal(out[1], nodes[1])
	require.Equal(out[1].FinalScore, 0.0)
}

func TestNodeAffinityIterator(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{Node: mock.Node()},
		{Node: mock.Node()},
		{Node: mock.Node()},
		{Node: mock.Node()},
	}

	nodes[0].Node.Attributes["kernel.version"] = "4.9"
	nodes[1].Node.Datacenter = "dc2"
	nodes[2].Node.Datacenter = "dc2"
	nodes[2].Node.NodeClass = "large"

	affinities := []*structs.Affinity{
		{
			Operand: "=",
			LTarget: "${node.datacenter}",
			RTarget: "dc1",
			Weight:  100,
		},
		{
			Operand: "=",
			LTarget: "${node.datacenter}",
			RTarget: "dc2",
			Weight:  -100,
		},
		{
			Operand: "version",
			LTarget: "${attr.kernel.version}",
			RTarget: ">4.0",
			Weight:  50,
		},
		{
			Operand: "is",
			LTarget: "${node.class}",
			RTarget: "large",
			Weight:  50,
		},
	}

	static := NewStaticRankIterator(ctx, nodes)

	job := mock.Job()
	job.ID = "foo"
	tg := job.TaskGroups[0]
	tg.Affinities = affinities

	nodeAffinity := NewNodeAffinityIterator(ctx, static)
	nodeAffinity.SetTaskGroup(tg)

	scoreNorm := NewScoreNormalizationIterator(ctx, nodeAffinity)

	out := collectRanked(scoreNorm)
	expectedScores := make(map[string]float64)
	// Total weight = 300
	// Node 0 matches two affinities(dc and kernel version), total weight = 150
	expectedScores[nodes[0].Node.ID] = 0.5

	// Node 1 matches an anti affinity, weight = -100
	expectedScores[nodes[1].Node.ID] = -(1.0 / 3.0)

	// Node 2 matches one affinity(node class) with weight 50
	expectedScores[nodes[2].Node.ID] = -(1.0 / 6.0)

	// Node 3 matches one affinity (dc) with weight = 100
	expectedScores[nodes[3].Node.ID] = 1.0 / 3.0

	require := require.New(t)
	for _, n := range out {
		require.Equal(expectedScores[n.Node.ID], n.FinalScore)
	}

}
