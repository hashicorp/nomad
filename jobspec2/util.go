// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec2

// pointerOf returns a pointer to "a". It is duplicated from the helper package
// to isolate the jobspec2 package from the rest of Nomad.
func pointerOf[A any](a A) *A {
	return &a
}
