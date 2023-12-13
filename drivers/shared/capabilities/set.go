// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package capabilities is used for managing sets of linux capabilities.
package capabilities

import (
	"sort"
	"strings"
)

type nothing struct{}

var null = nothing{}

// Set represents a group linux capabilities, implementing some useful set
// operations, taking care of name normalization, and sentinel value expansions.
//
// Linux capabilities can be expressed in multiple ways when working with docker
// and/or executor, along with Nomad configuration.
//
// Capability names may be upper or lower case, and may or may not be prefixed
// with "CAP_" or "cap_". On top of that, Nomad interprets the special name "all"
// and "ALL" to mean "all capabilities supported by the operating system".
type Set struct {
	data map[string]nothing
}

// New creates a new Set setting caps as the initial elements.
func New(caps []string) *Set {
	m := make(map[string]nothing, len(caps))
	for _, c := range caps {
		insert(m, c)
	}
	return &Set{data: m}
}

// Add cap into s.
func (s *Set) Add(cap string) {
	insert(s.data, cap)
}

func insert(data map[string]nothing, cap string) {
	switch name := normalize(cap); name {
	case "":
	case "all":
		for k, v := range Supported().data {
			data[k] = v
		}
		return
	default:
		data[name] = null
	}
}

// Remove caps from s.
func (s *Set) Remove(caps []string) {
	for _, c := range caps {
		name := normalize(c)
		if name == "all" {
			s.data = make(map[string]nothing)
			return
		}
		delete(s.data, name)
	}
}

// Union returns of Set of elements of both s and b.
func (s *Set) Union(b *Set) *Set {
	data := make(map[string]nothing)
	for c := range s.data {
		data[c] = null
	}
	for c := range b.data {
		data[c] = null
	}
	return &Set{data: data}
}

// Difference returns the Set of elements of b not in s.
func (s *Set) Difference(b *Set) *Set {
	data := make(map[string]nothing)
	for c := range b.data {
		if _, exists := s.data[c]; !exists {
			data[c] = null
		}
	}
	return &Set{data: data}
}

// Intersect returns the Set of elements in both s and b.
func (s *Set) Intersect(b *Set) *Set {
	data := make(map[string]nothing)
	for c := range s.data {
		if _, exists := b.data[c]; exists {
			data[c] = null
		}
	}
	return &Set{data: data}
}

// Empty return true if no capabilities exist in s.
func (s *Set) Empty() bool {
	return len(s.data) == 0
}

// String returns the normalized and sorted string representation of s.
func (s *Set) String() string {
	return strings.Join(s.Slice(false), ", ")
}

// Slice returns a sorted slice of capabilities in s.
//
// upper - indicates whether to uppercase and prefix capabilities with CAP_
func (s *Set) Slice(upper bool) []string {
	caps := make([]string, 0, len(s.data))
	for c := range s.data {
		if upper {
			c = "CAP_" + strings.ToUpper(c)
		}
		caps = append(caps, c)
	}
	sort.Strings(caps)
	return caps
}

// linux capabilities are often named in 4 possible ways - upper or lower case,
// and with or without a CAP_ prefix
//
// since we must do comparisons on cap names, always normalize the names before
// letting them into the Set data-structure
func normalize(name string) string {
	spaces := strings.TrimSpace(name)
	lower := strings.ToLower(spaces)
	trim := strings.TrimPrefix(lower, "cap_")
	return trim
}
