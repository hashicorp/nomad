package fixedheap

import (
	"github.com/google/btree"
)

type FixedHeap[T any] struct {
	tree *btree.BTreeG[T]
	max  int
}

func NewFixedHeap[T any](max int, lessFn btree.LessFunc[T]) FixedHeap[T] {
	return FixedHeap[T]{
		tree: btree.NewG[T](64, lessFn),
		max:  max,
	}
}

func (f *FixedHeap[T]) Push(item T) (T, bool) {
	if f.tree.Len() < f.max {
		f.tree.ReplaceOrInsert(item)
		return item, true
	}

	var result T
	f.tree.AscendLessThan(item, func(other T) bool {
		result = other
		return true
	})

	f.tree.Delete(result)
	f.tree.ReplaceOrInsert(item)
	return result, false
}

func (f *FixedHeap[T]) Pop() (T, bool) {
	var result T
	item, ok := f.tree.Min()
	if !ok {
		return result, false
	}
	f.tree.Delete(item)
	return item, true
}

func (f *FixedHeap[T]) Peek() (T, bool) {
	return f.tree.Min()
}

// PeekN returns the item at the zero-indexed index value
func (f *FixedHeap[T]) PeekN(index int) (T, bool) {
	var result T
	var ok bool
	var count int
	f.tree.Descend(func(item T) bool {
		if count == index {
			result = item
			ok = true
			return false
		}
		count++
		return true
	})
	return result, ok
}

func (f *FixedHeap[T]) Remove(item T) (T, bool) {
	return f.tree.Delete(item)
}

func (f *FixedHeap[T]) Slice() []T {
	orderedItems := []T{}
	f.tree.Descend(func(item T) bool {
		orderedItems = append(orderedItems, item)
		return true
	})
	return orderedItems
}

func (f *FixedHeap[T]) SliceReverse() []T {
	orderedItems := []T{}
	f.tree.Ascend(func(item T) bool {
		orderedItems = append(orderedItems, item)
		return true
	})
	return orderedItems
}

func (f *FixedHeap[T]) Iter() *FixedHeapIterator[T] {
	return &FixedHeapIterator[T]{
		cursor: 0,
		heap:   f,
	}
}

func (f *FixedHeap[T]) Len() int {
	if f == nil || f.tree == nil {
		return 0
	}
	return f.tree.Len()
}

type FixedHeapIterator[T any] struct {
	cursor int
	heap   *FixedHeap[T]
}

func (iter *FixedHeapIterator[T]) Next() (T, bool) {
	item, ok := iter.heap.PeekN(iter.cursor)
	iter.cursor++
	return item, ok
}
