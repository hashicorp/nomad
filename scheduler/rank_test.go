package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestFeasibleRankIterator(t *testing.T) {
	ctx := NewEvalContext()
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	feasible := NewFeasibleRankIterator(ctx, static)

	var out []*RankedNode
	for {
		next := feasible.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}

	if len(out) != len(nodes) {
		t.Fatalf("bad: %v", out)
	}
}

func TestScoreFit(t *testing.T) {
	node := mock.Node()
	node.Resources = &structs.Resources{
		CPU:      4096,
		MemoryMB: 8192,
	}
	node.Reserved = &structs.Resources{
		CPU:      2048,
		MemoryMB: 4096,
	}

	// Test a perfect fit
	util := &structs.Resources{
		CPU:      2048,
		MemoryMB: 4096,
	}
	score := scoreFit(node, util)
	if score != 18.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test the worst fit
	util = &structs.Resources{
		CPU:      0,
		MemoryMB: 0,
	}
	score = scoreFit(node, util)
	if score != 0.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test a mid-case scenario
	util = &structs.Resources{
		CPU:      1024,
		MemoryMB: 2048,
	}
	score = scoreFit(node, util)
	if score < 10.0 || score > 16.0 {
		t.Fatalf("bad: %v", score)
	}
}
