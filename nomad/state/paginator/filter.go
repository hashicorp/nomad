// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

// FilterFunc is an interface for functions that return true if the object
// evaluated should be included in the page.
//
// Warning: this is the opposite of a memdb.FilterFunc, where returning true
// excludes the object!
type FilterFunc[T any] func(T) bool

func NamespaceFilterFunc[T namespaceGetter](allowedNS map[string]bool) func(obj T) bool {
	return func(obj T) bool {
		if allowedNS == nil {
			return true // management tokens always have nil here
		}
		ns := obj.GetNamespace()
		return allowedNS[ns]
	}
}
