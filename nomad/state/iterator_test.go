// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/nomad/v2/ci"
	"github.com/shoenig/test/must"
)

func TestSliceIterator(t *testing.T) {
	ci.Parallel(t)

	sliceIterator := NewSliceIterator()
	must.NotNil(t, sliceIterator)

	// Add something and perform our tests to ensure the expected data is
	// returned.
	sliceIterator.Add("random-information")
	must.Len(t, 1, sliceIterator.data)
	must.Zero(t, sliceIterator.idx)
	must.Nil(t, sliceIterator.WatchCh())

	next1 := sliceIterator.Next()
	next2 := sliceIterator.Next()
	must.Eq(t, "random-information", next1)
	must.Nil(t, next2)
	must.Eq(t, 1, sliceIterator.idx)
}
