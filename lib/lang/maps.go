// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

import (
	"cmp"
	"slices"
)

// WalkMap will call f for every k/v in m, iterating the keyset of m in the
// cmp.Ordered order. If f returns false the iteration is halted early.
func WalkMap[K cmp.Ordered, V any](m map[K]V, f func(K, V) bool) {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// sort keys ascending
	slices.Sort(keys)

	for _, k := range keys {
		if !f(k, m[k]) {
			return // stop iteration
		}
	}
}
