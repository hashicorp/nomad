// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package safemath

import (
	"math"
	"testing"

	"github.com/shoenig/test/must"
)

// TestAdd_Overflow
func TestAdd_Overflow(t *testing.T) {
	must.Eq(t, math.MaxInt64, Add(math.MaxInt64, math.MaxInt64)) // overflow
	must.Eq(t, math.MaxInt64, Add(1, math.MaxInt64))             // overflow (boundary)
	must.Eq(t, math.MaxInt64-1, Add(-1, math.MaxInt64))          // no overflow (boundary)
	must.Eq(t, -1, Add(math.MaxInt64-1, -math.MaxInt64))         // no overflow (subtraction)
	up := int64(1)
	must.Eq(t, math.MaxInt64, Add(math.MaxInt64+up, -math.MaxInt64)) // operand overflowed
}
