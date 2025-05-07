// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"iter"

	memdb "github.com/hashicorp/go-memdb"
)

type ResultIterator[T comparable] interface {
	WatchCh() <-chan struct{}
	Next() T
	All() iter.Seq[T]
	Slice() []T
}

type typedResultIterator[T comparable] struct {
	inner memdb.ResultIterator
}

func NewResultIterator[T comparable](inner memdb.ResultIterator) ResultIterator[T] {
	return typedResultIterator[T]{
		inner: inner,
	}
}

func (iter typedResultIterator[T]) Next() T {
	raw := iter.inner.Next()
	if raw == nil {
		return *(new(T)) // zero value
	}
	return raw.(T)
}

func (iter typedResultIterator[T]) WatchCh() <-chan struct{} {
	return iter.inner.WatchCh()
}

func (iter typedResultIterator[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			raw := iter.inner.Next()
			if raw == nil {
				return
			}
			v := raw.(T)
			if !yield(v) {
				return
			}
		}
	}
}

func (iter typedResultIterator[T]) Slice() []T {
	out := []T{}
	for v := range iter.All() {
		out = append(out, v)
	}
	return out
}

type FilterFunc[T comparable] func(T) bool

type typedFilterResultIterator[T comparable] struct {
	inner      ResultIterator[T]
	filterFunc FilterFunc[T]
}

func NewFilterIterator[T comparable](iter ResultIterator[T], filterFunc FilterFunc[T]) ResultIterator[T] {
	return typedFilterResultIterator[T]{
		inner:      iter,
		filterFunc: filterFunc,
	}
}

func (iter typedFilterResultIterator[T]) Next() T {
	for {
		val := iter.inner.Next()
		if val == *new(T) {
			return val
		}
		if !iter.filterFunc(val) {
			return val
		}
	}
}

func (iter typedFilterResultIterator[T]) WatchCh() <-chan struct{} {
	return iter.inner.WatchCh()
}

func (iter typedFilterResultIterator[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			val := iter.inner.Next()
			if val == *new(T) {
				break
			}
			if iter.filterFunc(val) {
				continue
			}
			if !yield(val) {
				return
			}
		}
	}
}

func (iter typedFilterResultIterator[T]) Slice() []T {
	out := []T{}
	for v := range iter.All() {
		out = append(out, v)
	}
	return out
}

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
