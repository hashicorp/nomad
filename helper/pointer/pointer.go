// Package pointer provides helper functions related to Go pointers.
package pointer

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
