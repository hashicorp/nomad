package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
)

func TestLimitIterator(t *testing.T) {
	ctx := NewEvalContext()
	nodes := []*RankedNode{
		&RankedNode{
			Node:  mock.Node(),
			Score: 1,
		},
		&RankedNode{
			Node:  mock.Node(),
			Score: 2,
		},
		&RankedNode{
			Node:  mock.Node(),
			Score: 3,
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	limit := NewLimitIterator(ctx, static, 2)

	var out []*RankedNode
	for {
		next := limit.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}

	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[0] && out[1] != nodes[1] {
		t.Fatalf("bad: %v", out)
	}
}

func TestMaxScoreIterator(t *testing.T) {
	ctx := NewEvalContext()
	nodes := []*RankedNode{
		&RankedNode{
			Node:  mock.Node(),
			Score: 1,
		},
		&RankedNode{
			Node:  mock.Node(),
			Score: 2,
		},
		&RankedNode{
			Node:  mock.Node(),
			Score: 3,
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	max := NewMaxScoreIterator(ctx, static)

	var out []*RankedNode
	for {
		next := max.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}

	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[2] {
		t.Fatalf("bad: %v", out)
	}
}
