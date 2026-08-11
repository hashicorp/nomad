package btree

import (
	"container/heap"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/hashicorp/go-set/v3"
)

type SimpleItem struct {
	*Item
}

func (i SimpleItem) LessThan(o SimpleItem) bool {
	return i.key < o.key
}

func CompareFunc(i, j SimpleItem) int {
	return i.key - j.key
}

type Item struct {
	key   int
	value string
}

func benchmarkBpTreeInsertN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	b.ResetTimer()
	for b.Loop() {
		tree := NewBpTree[SimpleItem](100)
		for _, item := range items {
			tree.Insert(item)
		}
	}
}

func benchmarkTreeSetInsertN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	for b.Loop() {
		tree := set.NewTreeSet(CompareFunc)
		for _, item := range items {
			tree.Insert(item)
		}
	}
}

func benchmarkBpTreeRangeN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	tree := NewBpTree[SimpleItem](100)
	for _, item := range items {
		tree.Insert(item)
	}

	rangeStart := n / 10
	rangeEnd := rangeStart + 100

	b.ResetTimer()
	for b.Loop() {
		_ = tree.Range(rangeStart, rangeEnd)
	}
}

func benchmarkTreeSetRangeN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	tree := set.NewTreeSet(CompareFunc)
	for _, item := range items {
		tree.Insert(item)
	}

	rangeStart := n / 10
	rangeEnd := rangeStart + 100

	b.ResetTimer()
	for b.Loop() {
		s := tree.Slice()
		_ = s[rangeStart : rangeEnd+1]
	}
}

func benchmarkPriorityQueueInsertN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	for b.Loop() {
		pq := make(PriorityQueue, 0, n)
		heap.Init(&pq)
		for _, item := range items {
			heap.Push(&pq, item)
		}
	}
}

func benchmarkPriorityQueueRangeN(b *testing.B, n int) {
	items := make([]SimpleItem, n)
	for i := range items {
		items[i] = SimpleItem{
			&Item{
				key:   rand.IntN(n * 10),
				value: fmt.Sprintf("item-%d", i),
			},
		}
	}

	pq := make(PriorityQueue, 0, n)
	heap.Init(&pq)
	for _, item := range items {
		heap.Push(&pq, item)
	}

	rangeStart := n / 10
	rangeEnd := rangeStart + 100

	b.ResetTimer()
	for b.Loop() {
		s := make([]SimpleItem, len(pq))
		copy(s, pq)
		sort.Slice(s, func(i, j int) bool {
			return s[i].LessThan(s[j])
		})
		_ = s[rangeStart : rangeEnd+1]
	}
}

// PriorityQueue implements heap.Interface for SimpleItem
type PriorityQueue []SimpleItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].LessThan(pq[j])
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x any) {
	*pq = append(*pq, x.(SimpleItem))
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// BenchmarkInsert runs sub-benchmarks for Insert operations across different sizes
func BenchmarkInsert(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("BpTree/%d", size), func(b *testing.B) {
			benchmarkBpTreeInsertN(b, size)
		})

		b.Run(fmt.Sprintf("TreeSet/%d", size), func(b *testing.B) {
			benchmarkTreeSetInsertN(b, size)
		})

		b.Run(fmt.Sprintf("PQ/%d", size), func(b *testing.B) {
			benchmarkPriorityQueueInsertN(b, size)
		})
		fmt.Printf("\n")
	}
}

// BenchmarkRange runs sub-benchmarks for Range operations across different sizes
func BenchmarkRange(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("BpTree/%d", size), func(b *testing.B) {
			benchmarkBpTreeRangeN(b, size)
		})

		b.Run(fmt.Sprintf("TreeSet/%d", size), func(b *testing.B) {
			benchmarkTreeSetRangeN(b, size)
		})

		b.Run(fmt.Sprintf("PQ/%d", size), func(b *testing.B) {
			benchmarkPriorityQueueRangeN(b, size)
		})
		fmt.Printf("\n")
	}
}
