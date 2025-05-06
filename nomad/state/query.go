// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"fmt"
)

var errMemDBInvariant = errors.New("memdb invariant failure")

func Get[T comparable](txn ReadTxn, table, index string, args ...any) ResultIterator[T] {
	iter, err := txn.Get(table, index, args...)
	if err != nil {
		panic(fmt.Errorf("%w: %w", errMemDBInvariant, err))
	}
	return NewResultIterator[T](iter)
}

func GetReverse[T comparable](txn ReadTxn, table, index string, args ...any) ResultIterator[T] {
	iter, err := txn.GetReverse(table, index, args...)
	if err != nil {
		panic(fmt.Errorf("%w: %w", errMemDBInvariant, err))
	}
	return NewResultIterator[T](iter)
}

func First[T comparable](txn ReadTxn, table, index string, args ...any) T {
	raw, err := txn.First(table, index, args...)
	if err != nil {
		panic(fmt.Errorf("%w: %w", errMemDBInvariant, err))
	}
	out := raw.(T)
	return out
}

func FirstWatch[T comparable](txn ReadTxn, table, index string, args ...any) (<-chan struct{}, T) {
	ch, raw, err := txn.FirstWatch(table, index, args...)
	if err != nil {
		panic(fmt.Errorf("%w: %w", errMemDBInvariant, err))
	}
	out := raw.(T)
	return ch, out
}
