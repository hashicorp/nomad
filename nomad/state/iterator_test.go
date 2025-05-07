// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestSliceIterator(t *testing.T) {
	ci.Parallel(t)

	sliceIterator := NewSliceIterator[*string]()
	must.NotNil(t, sliceIterator)

	// Add something and perform our tests to ensure the expected data is
	// returned.
	sliceIterator.Add(pointer.Of("random-information"))
	must.Len(t, 1, sliceIterator.data)
	must.Zero(t, sliceIterator.idx)
	must.Nil(t, sliceIterator.WatchCh())

	next1 := sliceIterator.Next()
	next2 := sliceIterator.Next()
	must.NotNil(t, next1)
	must.Eq(t, "random-information", *next1)
	must.Nil(t, next2)
	must.Eq(t, 1, sliceIterator.idx)
}
