// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

type SliceIterator struct {
	data []interface{}
	idx  int
}

func NewSliceIterator() *SliceIterator {
	return &SliceIterator{
		data: []interface{}{},
		idx:  0,
	}
}

func (i *SliceIterator) Add(datum interface{}) {
	i.data = append(i.data, datum)
}

func (i *SliceIterator) Next() interface{} {
	if i.idx == len(i.data) {
		return nil
	}

	datum := i.data[i.idx]
	i.idx += 1
	return datum
}

func (i *SliceIterator) WatchCh() <-chan struct{} {
	return nil
}
