// Package pointer provides helper functions related to Go pointers.
package pointer

import (
	"golang.org/x/exp/constraints"
)

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

// Primitive represents basic types that are safe to do basic comparisons by
// pointer dereference (checking nullity first).
type Primitive interface {
	constraints.Ordered // just so happens to be the types we want
}

// Eq returns whether a and b are equal in underlying value.
//
// May only be used on pointers to primitive types, where the comparison is
// guaranteed to be sensible. For complex types (i.e. structs) consider implementing
// an Equals method.
func Eq[P Primitive](a, b *P) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
