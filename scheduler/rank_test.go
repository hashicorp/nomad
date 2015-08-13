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

func TestBinPackIterator(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: mock.Node(),
		},
		&RankedNode{
			Node: mock.Node(),
		},
		&RankedNode{
			Node: mock.Node(),
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	alloc := mock.Alloc()
	resources := alloc.Resources
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

	out := collectRanked(binp)
	if len(out) != 3 {
		t.Fatalf("Bad: %v", out)
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
