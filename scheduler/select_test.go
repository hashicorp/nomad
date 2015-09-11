package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
)

func TestLimitIterator(t *testing.T) {
	_, ctx := testContext(t)
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

	limit := NewLimitIterator(ctx, static, 1)
	limit.SetLimit(2)

	out := collectRanked(limit)
	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[0] && out[1] != nodes[1] {
		t.Fatalf("bad: %v", out)
	}

	out = collectRanked(limit)
	if len(out) != 0 {
		t.Fatalf("bad: %v", out)
	}
	limit.Reset()

	out = collectRanked(limit)
	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[2] && out[1] != nodes[0] {
		t.Fatalf("bad: %v", out)
	}
}

func TestMaxScoreIterator(t *testing.T) {
	_, ctx := testContext(t)
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

	out := collectRanked(max)
	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[2] {
		t.Fatalf("bad: %v", out)
	}

	out = collectRanked(max)
	if len(out) != 0 {
		t.Fatalf("bad: %v", out)
	}
	max.Reset()

	out = collectRanked(max)
	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != nodes[2] {
		t.Fatalf("bad: %v", out)
	}
}
