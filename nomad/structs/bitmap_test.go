// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestBitmap(t *testing.T) {
	ci.Parallel(t)

	// Check invalid sizes
	_, err := NewBitmap(0)
	must.Error(t, err)
	_, err = NewBitmap(7)
	must.Error(t, err)

	// Create a normal bitmap
	var s uint = 256
	b, err := NewBitmap(s)
	must.NoError(t, err)
	must.Eq(t, s, b.Size())

	// Set a few bits
	b.Set(0)
	b.Set(255)

	// Verify the bytes
	must.NotEq(t, 0, b[0])
	must.True(t, b.Check(0))

	// Verify the bytes
	must.NotEq(t, 0, b[len(b)-1])
	must.True(t, b.Check(255))

	// All other bits should be unset
	for i := 1; i < 255; i++ {
		must.False(t, b.Check(uint(i)))
	}

	// Check the indexes
	idxs := b.IndexesInRange(true, 0, 500)
	must.Eq(t, []int{0, 255}, idxs)

	idxs = b.IndexesInRange(true, 1, 255)
	must.Eq(t, []int{255}, idxs)

	idxs = b.IndexesInRange(false, 0, 256)
	must.Len(t, 254, idxs)

	idxs = b.IndexesInRange(false, 100, 200)
	must.Len(t, 101, idxs)

	// Check the copy is correct
	b2, err := b.Copy()
	must.NoError(t, err)
	must.Eq(t, b, b2)

	// Clear
	b.Clear()

	// All bits should be unset
	for i := 0; i < 256; i++ {
		must.False(t, b.Check(uint(i)))
	}

	// Set a few bits
	b.Set(0)
	b.Set(255)
	b.Unset(0)
	b.Unset(255)

	// Clear the bits
	must.Eq(t, 0, b[0])
	must.False(t, b.Check(0))

	// Verify the bytes
	must.Eq(t, 0, b[len(b)-1])
	must.False(t, b.Check(255))
}
