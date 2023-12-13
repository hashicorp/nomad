// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/exp/maps"
)

var (
	cmpOptIgnoreUnexported = ignoreUnexportedAlways()
	cmpOptNilIsEmpty       = cmpopts.EquateEmpty()
	cmpOptIgnore           = cmp.Ignore()
)

// ignoreUnexportedAlways creates a cmp.Option filter that will ignore unexported
// fields of on any/all types. It is a derivative of go-cmp.IgnoreUnexported,
// here we do not require specifying individual types.
//
// reference: https://github.com/google/go-cmp/blob/master/cmp/cmpopts/ignore.go#L110
func ignoreUnexportedAlways() cmp.Option {
	return cmp.FilterPath(
		func(p cmp.Path) bool {
			sf, ok := p.Index(-1).(cmp.StructField)
			if !ok {
				return false
			}
			c := sf.Name()[0]
			return c < 'A' || c > 'Z'
		},
		cmpOptIgnore,
	)
}

// OpaqueMapsEqual compare maps[<comparable>]<any> for equality, but safely by
// using the cmp package and ignoring un-exported types, and by treating nil/empty
// slices and maps as equal.
//
// This is intended as a substitute for reflect.DeepEqual in the case of "opaque maps",
// e.g. `map[comparable]any` - such as the case for Task Driver config or Envoy proxy
// pass-through configuration.
func OpaqueMapsEqual[M ~map[K]V, K comparable, V any](m1, m2 M) bool {
	return maps.EqualFunc(m1, m2, func(a, b V) bool {
		return cmp.Equal(a, b,
			cmpOptIgnoreUnexported, // ignore all unexported fields
			cmpOptNilIsEmpty,       // treat nil/empty slices as equal
		)
	})
}
