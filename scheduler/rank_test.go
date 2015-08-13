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

func TestBinPackIterator(t *testing.T) {
}
