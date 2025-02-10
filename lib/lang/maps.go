// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

import (
	"cmp"
	"slices"

	"github.com/hashicorp/nomad/helper/pointer"
)

// MapKeys will return a slice of keys of m in no particular order.
func MapKeys[M ~map[K]V, K pointer.Primitive, V any](m M) []K {
	result := make([]K, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	return result
}

// MapClear will delete all elements out of m.
func MapClear[M ~map[K]V, K pointer.Primitive, V any](m M) {
	for key := range m {
		delete(m, key)
	}
}

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
