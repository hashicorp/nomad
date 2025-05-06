// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import "iter"

type SliceIterator[T comparable] struct {
	data []T
	idx  int
}

func NewSliceIterator[T comparable]() *SliceIterator[T] {
	return &SliceIterator[T]{
		data: []T{},
		idx:  0,
	}
}

func (i *SliceIterator[T]) Add(datum T) {
	i.data = append(i.data, datum)
}

func (i *SliceIterator[T]) Next() T {
	if i.idx == len(i.data) {
		return *(new(T))
	}

	datum := i.data[i.idx]
	i.idx += 1
	return datum
}

func (i *SliceIterator[T]) WatchCh() <-chan struct{} {
	return nil
}

func (i *SliceIterator[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, val := range i.data {
			if !yield(val) {
				return
			}
		}
	}
}

func (i *SliceIterator[T]) Slice() []T {
	return i.data
}
