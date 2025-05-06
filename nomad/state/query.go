// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"
)

var ErrMemDBInvariant = errors.New("memdb invariant failure")

func Get[T comparable](txn ReadTxn, table, index string, args ...any) (ResultIterator[T], error) {
	iter, err := txn.Get(table, index, args...)
	if err != nil {
		return nil, err
	}
	return NewResultIterator[T](iter), nil
}

func GetSorted[T comparable](txn ReadTxn, sort SortOption, table, index string, args ...any) (ResultIterator[T], error) {
	var iter ResultIterator[T]
	var err error
	switch sort {
	case SortReverse:
		iter, err = GetReverse[T](txn, table, index, args...)
	default:
		iter, err = Get[T](txn, table, index, args...)
	}
	if err != nil {
		return nil, err
	}
	return iter, nil
}

func GetAll[T comparable](txn ReadTxn, sort SortOption, table, index string) ResultIterator[T] {
	iter, err := GetSorted[T](txn, sort, table, index)
	if err != nil {
		panic(fmt.Errorf("%w: %w", errIndexInvariant, err))
	}

	return iter
}

func GetReverse[T comparable](txn ReadTxn, table, index string, args ...any) (ResultIterator[T], error) {
	iter, err := txn.GetReverse(table, index, args...)
	if err != nil {
		return nil, err
	}
	return NewResultIterator[T](iter), nil
}

func First[T comparable](txn ReadTxn, table, index string, args ...any) (T, error) {
	raw, err := txn.First(table, index, args...)
	if err != nil {
		return *(new(T)), err
	}
	if raw == nil {
		return *(new(T)), nil
	}
	out := raw.(T)
	return out, nil
}

func FirstWatch[T comparable](txn ReadTxn, table, index string, args ...any) (<-chan struct{}, T, error) {
	ch, raw, err := txn.FirstWatch(table, index, args...)
	if err != nil {
		return ch, *(new(T)), err
	}
	if raw == nil {
		return ch, *(new(T)), nil
	}
	out := raw.(T)
	return ch, out, nil
}
