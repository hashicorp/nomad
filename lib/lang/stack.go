// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lang

// A Stack is a simple LIFO datastructure.
//
// This datastructure is not concurrency-safe; any locking
// required should be done by the caller!
type Stack[T any] struct {
	top *object[T]
}

type object[T any] struct {
	item T
	next *object[T]
}

// NewStack creates a Stack with no elements.
func NewStack[T any]() *Stack[T] {
	return new(Stack[T])
}

// Push pushes item onto the stack.
func (s *Stack[T]) Push(item T) {
	obj := &object[T]{
		item: item,
		next: s.top,
	}
	s.top = obj
}

// Pop pops the most recently pushed item from the Stack.
//
// It is a logic bug to Pop an Empty stack.
func (s *Stack[T]) Pop() T {
	obj := s.top
	s.top = obj.next
	obj.next = nil
	return obj.item
}

// Empty returns true if there are no elements on the Stack.
func (s *Stack[T]) Empty() bool {
	return s.top == nil
}
