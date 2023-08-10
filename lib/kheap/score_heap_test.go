// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package kheap

import (
	"container/heap"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

type heapItem struct {
	Value    string
	ScoreVal float64
}

func (h *heapItem) Data() interface{} {
	return h.Value
}

func (h *heapItem) Score() float64 {
	return h.ScoreVal
}

func TestScoreHeap(t *testing.T) {
	ci.Parallel(t)

	type testCase struct {
		desc     string
		items    map[string]float64
		expected []*heapItem
	}

	cases := []testCase{
		{
			desc: "More than K elements",
			items: map[string]float64{
				"banana":     3.0,
				"apple":      2.25,
				"pear":       2.32,
				"watermelon": 5.45,
				"orange":     0.20,
				"strawberry": 9.03,
				"blueberry":  0.44,
				"lemon":      3.9,
				"cherry":     0.03,
			},
			expected: []*heapItem{
				{Value: "pear", ScoreVal: 2.32},
				{Value: "banana", ScoreVal: 3.0},
				{Value: "lemon", ScoreVal: 3.9},
				{Value: "watermelon", ScoreVal: 5.45},
				{Value: "strawberry", ScoreVal: 9.03},
			},
		},
		{
			desc: "Less than K elements",
			items: map[string]float64{
				"eggplant": 9.0,
				"okra":     -1.0,
				"corn":     0.25,
			},
			expected: []*heapItem{
				{Value: "okra", ScoreVal: -1.0},
				{Value: "corn", ScoreVal: 0.25},
				{Value: "eggplant", ScoreVal: 9.0},
			},
		},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			// Create Score heap, push elements into it
			pq := NewScoreHeap(5)
			for value, score := range tc.items {
				heapItem := &heapItem{
					Value:    value,
					ScoreVal: score,
				}
				heap.Push(pq, heapItem)
			}

			// Take the items out; they arrive in increasing Score order
			require := require.New(t)
			require.Equal(len(tc.expected), pq.Len())

			i := 0
			for pq.Len() > 0 {
				item := heap.Pop(pq).(*heapItem)
				require.Equal(tc.expected[i], item)
				i++
			}
		})
	}

}
