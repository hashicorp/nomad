// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package iterator

// Iterator represents an object that can iterate over a set of values one at a
// time.
type Iterator interface {
	// Next returns the next element or nil if there are none left.
	Next() any
}

// Len consumes the iterator and returns the number of elements found.
//
// IMPORTANT: this method consumes the iterator, so it should not be used after
// Len() returns.
func Len(iter Iterator) int {
	count := 0
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++
	}
	return count
}
