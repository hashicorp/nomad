package lang

// Pair associates two arbitrary types together.
type Pair[T, U any] struct {
	First  T
	Second U
}
