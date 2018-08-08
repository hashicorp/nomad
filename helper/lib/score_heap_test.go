package lib

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScoreHeap(t *testing.T) {
	type testCase struct {
		desc     string
		items    map[string]float64
		expected []*HeapItem
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
			expected: []*HeapItem{
				{Value: "pear", Score: 2.32},
				{Value: "banana", Score: 3.0},
				{Value: "lemon", Score: 3.9},
				{Value: "watermelon", Score: 5.45},
				{Value: "strawberry", Score: 9.03},
			},
		},
		{
			desc: "Less than K elements",
			items: map[string]float64{
				"eggplant": 9.0,
				"okra":     -1.0,
				"corn":     0.25,
			},
			expected: []*HeapItem{
				{Value: "okra", Score: -1.0},
				{Value: "corn", Score: 0.25},
				{Value: "eggplant", Score: 9.0},
			},
		},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			// Create Score heap, push elements into it
			pq := NewScoreHeap(5)
			for value, score := range tc.items {
				heapItem := &HeapItem{
					Value: value,
					Score: score,
				}
				heap.Push(pq, heapItem)
			}

			// Take the items out; they arrive in increasing Score order
			require := require.New(t)
			require.Equal(len(tc.expected), pq.Len())

			i := 0
			for pq.Len() > 0 {
				item := heap.Pop(pq).(*HeapItem)
				require.Equal(tc.expected[i], item)
				i++
			}
		})
	}

}
