// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import memdb "github.com/hashicorp/go-memdb"

// IterCount is a helper that consumes an iterator and returns a count of the
// objects found in it
func (s *StateStore) IterCount(iter memdb.ResultIterator) int {
	count := 0
	for {
		if iter.Next() == nil {
			return count
		}
		count++
	}
}
