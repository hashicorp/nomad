// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lang

type Stack[T any] struct {
	top *object[T]
}

type object[T any] struct {
	item T
	next *object[T]
}

func NewStack[T any]() *Stack[T] {
	return new(Stack[T])
}

func (s *Stack[T]) Push(item T) {
	obj := &object[T]{
		item: item,
		next: s.top,
	}
	s.top = obj
}

func (s *Stack[T]) Pop() T {
	obj := s.top
	s.top = obj.next
	obj.next = nil
	return obj.item
}

func (s *Stack[T]) Empty() bool {
	return s.top == nil
}
