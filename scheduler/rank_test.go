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

func TestBinPackIterator_Simple_NoExistingAlloc(t *testing.T) {
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

	resources := &structs.Resources{
		CPU:      1024,
		MemoryMB: 1024,
	}
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

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
