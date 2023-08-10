// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Stack(t *testing.T) {
	s := NewStack[int]()

	must.True(t, s.Empty())

	s.Push(1)
	s.Push(2)
	s.Push(3)
	must.NotEmpty(t, s)

	must.Eq(t, 3, s.Pop())
	must.Eq(t, 2, s.Pop())

	s.Push(4)
	s.Push(5)

	must.Eq(t, 5, s.Pop())
	must.Eq(t, 4, s.Pop())
	must.Eq(t, 1, s.Pop())
	must.Empty(t, s)
}
