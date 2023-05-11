// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lang

// Pair associates two arbitrary types together.
type Pair[T, U any] struct {
	First  T
	Second U
}
