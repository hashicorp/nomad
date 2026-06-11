package helper

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
