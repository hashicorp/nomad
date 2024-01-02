// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

// Filter is the interface that must be implemented to skip values when using
// the Paginator.
type Filter interface {
	// Evaluate returns true if the element should be added to the page.
	Evaluate(interface{}) (bool, error)
}

// GenericFilter wraps a function that can be used to provide simple or in
// scope filtering.
type GenericFilter struct {
	Allow func(interface{}) (bool, error)
}

func (f GenericFilter) Evaluate(raw interface{}) (bool, error) {
	return f.Allow(raw)
}

// NamespaceFilter skips elements with a namespace value that is not in the
// allowable set.
type NamespaceFilter struct {
	AllowableNamespaces map[string]bool
}

func (f NamespaceFilter) Evaluate(raw interface{}) (bool, error) {
	if raw == nil {
		return false, nil
	}

	item, _ := raw.(NamespaceGetter)
	namespace := item.GetNamespace()

	if f.AllowableNamespaces == nil {
		return true, nil
	}
	if f.AllowableNamespaces[namespace] {
		return true, nil
	}
	return false, nil
}
