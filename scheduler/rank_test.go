package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
				Reserved: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Overloaded
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
				Reserved: &structs.Resources{
					CPU:      512,
					MemoryMB: 512,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// 50% fit
				Resources: &structs.Resources{
					CPU:      4096,
					MemoryMB: 4096,
				},
				Reserved: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
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

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[2] {
		t.Fatalf("Bad: %v", out)
	}

	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
	if out[1].Score < 10 || out[1].Score > 16 {
		t.Fatalf("Bad: %v", out[1])
	}
}

func TestBinPackIterator_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      2048,
				MemoryMB: 2048,
			},
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      1024,
				MemoryMB: 1024,
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

	out := collectRanked(binp)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}

	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
}

func TestBinPackIterator_ExistingAlloc(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        structs.GenerateUUID(),
		EvalID:    structs.GenerateUUID(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        structs.GenerateUUID(),
		EvalID:    structs.GenerateUUID(),
		NodeID:    nodes[1].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
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

	out := collectRanked(binp)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
}

func TestBinPackIterator_ExistingAlloc_PlannedEvict(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        structs.GenerateUUID(),
		EvalID:    structs.GenerateUUID(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        structs.GenerateUUID(),
		EvalID:    structs.GenerateUUID(),
		NodeID:    nodes[1].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
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

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score < 10 || out[0].Score > 16 {
		t.Fatalf("Bad: %v", out[0])
	}
	if out[1].Score != 18 {
		t.Fatalf("Bad: %v", out[1])
	}
}

func TestJobAntiAffinity_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				ID: structs.GenerateUUID(),
			},
		},
		&RankedNode{
			Node: &structs.Node{
				ID: structs.GenerateUUID(),
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			ID:    structs.GenerateUUID(),
			JobID: "foo",
		},
		&structs.Allocation{
			ID:    structs.GenerateUUID(),
			JobID: "foo",
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			JobID: "bar",
		},
	}

	binp := NewJobAntiAffinityIterator(ctx, static, 5.0, "foo")

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score != -10.0 {
		t.Fatalf("Bad: %#v", out[0])
	}

	if out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[1].Score != 0.0 {
		t.Fatalf("Bad: %v", out[1])
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
