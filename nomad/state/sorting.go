// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
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
