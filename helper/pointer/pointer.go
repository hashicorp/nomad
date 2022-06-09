// Package pointer provides helper functions related to Go pointers.
package pointer

// Of returns a pointer to a.
func Of[A any](a A) *A {
	return &a
}
