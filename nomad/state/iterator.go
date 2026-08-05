// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

type SliceIterator struct {
	data []any
	idx  int
}

func NewSliceIterator() *SliceIterator {
	return &SliceIterator{
		data: []any{},
		idx:  0,
	}
}

func (i *SliceIterator) Add(datum any) {
	i.data = append(i.data, datum)
}

func (i *SliceIterator) Next() any {
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
