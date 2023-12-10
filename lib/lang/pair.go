// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

// Pair associates two arbitrary types together.
type Pair[T, U any] struct {
	First  T
	Second U
}
