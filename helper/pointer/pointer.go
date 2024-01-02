// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package pointer provides helper functions related to Go pointers.
package pointer

import (
	"golang.org/x/exp/constraints"
)

// Primitive represents basic types that are safe to do basic comparisons by
// pointer dereference (checking nullity first).
type Primitive interface {
	constraints.Ordered | bool
}

// Of returns a pointer to a.
func Of[A any](a A) *A {
	return &a
}

// Copy returns a new pointer to a.
func Copy[A any](a *A) *A {
	if a == nil {
		return nil
	}
	na := *a
	return &na
}

// Merge will return Copy(next) if next is not nil, otherwise return Copy(previous).
func Merge[P Primitive](previous, next *P) *P {
	if next != nil {
		return Copy(next)
	}
	return Copy(previous)
}

// Eq returns whether a and b are equal in underlying value.
//
// May only be used on pointers to primitive types, where the comparison is
// guaranteed to be sensible. For complex types (i.e. structs) consider implementing
// an Equal method.
func Eq[P Primitive](a, b *P) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
