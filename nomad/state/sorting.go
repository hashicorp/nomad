// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SortOption represents how results can be sorted.
type SortOption bool

const (
	// SortDefault indicates that the result should be returned using the
	// default go-memdb ResultIterator order.
	SortDefault SortOption = false

	// SortReverse indicates that the result should be returned using the
	// reversed go-memdb ResultIterator order.
	SortReverse SortOption = true
)

// QueryOptionSort returns the appropriate SortOption for given QueryOptions.
func QueryOptionSort(qo structs.QueryOptions) SortOption {
	return SortOption(qo.Reverse)
}

// getSorted executes either txn.Get() or txn.GetReverse()
// depending on the provided SortOption.
func getSorted(txn *txn, sort SortOption, table, index string, args ...any) (memdb.ResultIterator, error) {
	switch sort {
	case SortDefault:
		return txn.Get(table, index, args...)
	case SortReverse:
		return txn.GetReverse(table, index, args...)
	default:
		// this should never happen, since SortOption is bool
		return nil, fmt.Errorf("unknown sort option: %v", sort)
	}
}
