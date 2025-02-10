// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package idset provides a Set implementation for keeping track of various
// types of numeric IDs (e.g. CoreID, ProcessID, etc.).
package idset

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set/v3"
)

// An ID is representative of a non-negative identifier of something like
// a CPU core ID, a NUMA node ID, etc.
//
// See the hwids package for typical use cases.
type ID interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}

// A Set contains some IDs.
//
// See the List Format section of
// https://www.man7.org/linux/man-pages/man7/cpuset.7.html
// for more information on the syntax and utility of these sets.
type Set[T ID] struct {
	items *set.Set[T]
}

// Empty creates a fresh Set with no elements.
func Empty[T ID]() *Set[T] {
	return &Set[T]{
		items: set.New[T](0),
	}
}

// FromFunc creates a Set from the given values by first applying
// the conversion function.
func FromFunc[T ID, A any](values []A, convert func(value A) T) *Set[T] {
	return &Set[T]{
		items: set.FromFunc(values, convert),
	}
}

// Copy creates a deep copy of s.
func (s *Set[T]) Copy() *Set[T] {
	return &Set[T]{items: s.items.Copy()}
}

var (
	numberRe = regexp.MustCompile(`^\d+$`)
	spanRe   = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

func atoi[T ID](s string) T {
	i, _ := strconv.Atoi(s)
	return T(i)
}

func order[T ID](a, b T) (T, T) {
	if a < b {
		return a, b
	}
	return b, a
}

// Parse the given cpuset into a set.
//
// The input is assumed to be valid.
func Parse[T ID](list string) *Set[T] {
	result := Empty[T]()

	add := func(s string) {
		s = strings.TrimSpace(s)
		switch {
		case numberRe.MatchString(s):
			result.items.Insert(atoi[T](s))
		case spanRe.MatchString(s):
			values := spanRe.FindStringSubmatch(s)
			low, high := order(atoi[T](values[1]), atoi[T](values[2]))
			for i := low; i <= high; i++ {
				result.items.Insert(i)
			}
		}
	}

	pieces := strings.Split(list, ",")
	for _, piece := range pieces {
		add(piece)
	}

	return result
}

// From returns Set created from the given slice.
func From[T, U ID](slice []U) *Set[T] {
	result := Empty[T]()
	for _, item := range slice {
		result.items.Insert(T(item))
	}
	return result
}

// Difference returns the set of elements in s but not in other.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	diff := s.items.Difference(other.items)
	return &Set[T]{items: diff.(*set.Set[T])}
}

// Intersect returns the set of elements that are in both s and other.
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
	intersection := s.items.Intersect(other.items)
	return &Set[T]{items: intersection.(*set.Set[T])}
}

// Contains returns whether the Set contains item.
func (s *Set[T]) Contains(item T) bool {
	return s.items.Contains(item)
}

// Insert item into the Set.
func (s *Set[T]) Insert(item T) {
	s.items.Insert(item)
}

// Slice returns a slice copy of the Set.
func (s *Set[T]) Slice() []T {
	items := s.items.Slice()
	slices.Sort(items)
	return items
}

// InsertSet inserts all items of other into s.
func (s *Set[T]) InsertSet(other *Set[T]) {
	s.items.InsertSet(other.items)
}

// RemoveSet removes all items of other from s.
func (s *Set[T]) RemoveSet(other *Set[T]) {
	s.items.RemoveSet(other.items)
}

// String creates a well-formed cpuset string representation of the Set.
func (s *Set[T]) String() string {
	if s.items.Empty() {
		// cgroups notation uses a space (or newline) to indicate
		// "empty"; and this value is written to cgroups interface
		// files
		const empty = " "
		return empty
	}

	var parts []string
	ids := s.Slice()

	low, high := ids[0], ids[0]
	for i := 1; i < len(ids); i++ {
		switch {
		case ids[i] == high+1:
			high = ids[i]
			continue
		case low == high:
			parts = append(parts, fmt.Sprintf("%d", low))
		default:
			parts = append(parts, fmt.Sprintf("%d-%d", low, high))
		}
		low, high = ids[i], ids[i] // new range
	}

	if low == high {
		parts = append(parts, fmt.Sprintf("%d", low))
	} else {
		parts = append(parts, fmt.Sprintf("%d-%d", low, high))
	}

	return strings.Join(parts, ",")
}

// ForEach iterates the elements in the set and applies f. Iteration stops
// if the result of f is a non-nil error.
func (s *Set[T]) ForEach(f func(id T) error) error {
	for id := range s.items.Items() {
		if err := f(id); err != nil {
			return err
		}
	}
	return nil
}

// Size returns the number of elements in the Set.
func (s *Set[T]) Size() int {
	return s.items.Size()
}

// Empty returns whether the set is empty.
func (s *Set[T]) Empty() bool {
	if s == nil || s.items == nil {
		return true
	}
	return s.items.Empty()
}

// InsertSlice is used to bludgen a slice of integers into s.
func InsertSlice[T ID, X ~uint16](s *Set[T], items ...X) {
	for _, item := range items {
		s.Insert(T(item))
	}
}

// Superset returns true of s is a superset of other.
func (s *Set[T]) Superset(other *Set[T]) bool {
	// todo(shoenig) optimize
	return s.items.ContainsSlice(other.items.Slice())
}
