package helper

import "github.com/hashicorp/go-set/v3"

// ConvertSlice takes the input slice and generates a new one using the
// supplied conversion function to covert the element. This is useful when
// converting a slice of strings to a slice of structs which wraps the string.
func ConvertSlice[A, B any](original []A, conversion func(a A) B) []B {
	result := make([]B, len(original))
	for i, element := range original {
		result[i] = conversion(element)
	}
	return result
}

// SliceSetEq returns true if slices a and b contain the same elements (in no
// particular order), using '==' for comparison.
//
// Note: for pointers, consider implementing an Equal method and using
// ElementsEqual instead.
func SliceSetEq[T comparable](a, b []T) bool {
	lenA, lenB := len(a), len(b)
	if lenA != lenB {
		return false
	}

	if lenA > 10 {
		// avoid quadratic comparisons over large input
		return set.From(a).EqualSlice(b)
	}

OUTER:
	for _, item := range a {
		for _, other := range b {
			if item == other {
				continue OUTER
			}
		}
		return false
	}
	return true
}
