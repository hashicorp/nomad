// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestLimitIterator(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node:       mock.Node(),
			FinalScore: 1,
		},
		{
			Node:       mock.Node(),
			FinalScore: 2,
		},
		{
			Node:       mock.Node(),
			FinalScore: 3,
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	limit := NewLimitIterator(ctx, static, 1, 0, 2)
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

func TestLimitIterator_ScoreThreshold(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	type testCase struct {
		desc        string
		nodes       []*RankedNode
		expectedOut []*RankedNode
		threshold   float64
		limit       int
		maxSkip     int
	}

	var nodes []*structs.Node
	for i := 0; i < 5; i++ {
		nodes = append(nodes, mock.Node())
	}

	testCases := []testCase{
		{
			desc: "Skips one low scoring node",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: 2,
				},
				{
					Node:       nodes[2],
					FinalScore: 3,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[1],
					FinalScore: 2,
				},
				{
					Node:       nodes[2],
					FinalScore: 3,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "Skips maxSkip scoring nodes",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: -2,
				},
				{
					Node:       nodes[2],
					FinalScore: 3,
				},
				{
					Node:       nodes[3],
					FinalScore: 4,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[2],
					FinalScore: 3,
				},
				{
					Node:       nodes[3],
					FinalScore: 4,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "maxSkip limit reached",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: -6,
				},
				{
					Node:       nodes[2],
					FinalScore: -3,
				},
				{
					Node:       nodes[3],
					FinalScore: -4,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[2],
					FinalScore: -3,
				},
				{
					Node:       nodes[3],
					FinalScore: -4,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "draw both from skipped nodes",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: -6,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: -6,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		}, {
			desc: "one node above threshold, one skipped node",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: 5,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[1],
					FinalScore: 5,
				},
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "low scoring nodes interspersed",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
				{
					Node:       nodes[1],
					FinalScore: 5,
				},
				{
					Node:       nodes[2],
					FinalScore: -2,
				},
				{
					Node:       nodes[3],
					FinalScore: 2,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[1],
					FinalScore: 5,
				},
				{
					Node:       nodes[3],
					FinalScore: 2,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "only one node, score below threshold",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -1,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   2,
		},
		{
			desc: "maxSkip is more than available nodes",
			nodes: []*RankedNode{
				{
					Node:       nodes[0],
					FinalScore: -2,
				},
				{
					Node:       nodes[1],
					FinalScore: 1,
				},
			},
			expectedOut: []*RankedNode{
				{
					Node:       nodes[1],
					FinalScore: 1,
				},
				{
					Node:       nodes[0],
					FinalScore: -2,
				},
			},
			threshold: -1,
			limit:     2,
			maxSkip:   10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			static := NewStaticRankIterator(ctx, tc.nodes)

			limit := NewLimitIterator(ctx, static, 1, 0, 2)
			limit.SetLimit(2)
			out := collectRanked(limit)
			require := require.New(t)
			require.Equal(tc.expectedOut, out)

			limit.Reset()
			require.Equal(0, limit.skippedNodeIndex)
			require.Equal(0, len(limit.skippedNodes))
		})
	}

}

func TestMaxScoreIterator(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node:       mock.Node(),
			FinalScore: 1,
		},
		{
			Node:       mock.Node(),
			FinalScore: 2,
		},
		{
			Node:       mock.Node(),
			FinalScore: 3,
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
