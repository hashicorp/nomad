// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"strconv"
	"strings"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocsFrom returns allocations in creation order, starting inclusively at the
// pagination token. The token need not identify an existing allocation. Invalid
// tokens retain the paginator's existing comparison-based fallback behavior.
func (s *StateStore) AllocsFrom(ws memdb.WatchSet, namespace, nextToken string, sort SortOption) (memdb.ResultIterator, error) {
	indexString, id, ok := strings.Cut(nextToken, ".")
	createIndex, err := strconv.Atoi(indexString)
	if !ok || err != nil || strings.ContainsRune(id, '\x00') {
		if namespace == structs.AllNamespacesSentinel {
			return s.Allocs(ws, sort)
		}
		return s.AllocsByNamespaceOrdered(ws, namespace, sort)
	}

	txn := s.db.ReadTxn()
	index := "create"
	var watchArgs []any
	seekArgs := []any{uint64(createIndex), id}
	if namespace != structs.AllNamespacesSentinel {
		index = "namespace_create"
		watchArgs = []any{terminate(namespace)}
		seekArgs = append([]any{namespace}, seekArgs...)
	}

	// Lower-bound iterators cannot be watched. Keep watching the original
	// index range in the same transaction so blocking queries still wake up
	// for updates, including when the requested page is empty.
	watchIndex := index
	if len(watchArgs) > 0 {
		watchIndex += "_prefix"
	}
	watch, err := getSorted(txn, sort, "allocs", watchIndex, watchArgs...)
	if err != nil {
		return nil, err
	}
	ws.Add(watch.WatchCh())

	var iter memdb.ResultIterator
	if sort == SortReverse {
		iter, err = txn.ReverseLowerBound("allocs", index, seekArgs...)
	} else {
		iter, err = txn.LowerBound("allocs", index, seekArgs...)
	}
	if err != nil {
		return nil, err
	}

	return &paginationIterator{
		ResultIterator: iter,
		watchCh:        watch.WatchCh(),
		within: func(raw any) bool {
			return namespace == structs.AllNamespacesSentinel || raw.(*structs.Allocation).Namespace == namespace
		},
	}, nil
}

// NodesFrom returns nodes in ID order, starting inclusively at nextToken.
func (s *StateStore) NodesFrom(ws memdb.WatchSet, nextToken string) (memdb.ResultIterator, error) {
	// UUID indexes use binary keys, whereas the paginator compares strings.
	// Only seek canonical tokens; preserve string comparison for other input.
	if !helper.IsUUID(nextToken) || strings.ToLower(nextToken) != nextToken {
		return s.Nodes(ws)
	}
	txn := s.db.ReadTxn()
	watch, err := txn.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(watch.WatchCh())
	iter, err := txn.LowerBound("nodes", "id", nextToken)
	if err != nil {
		return nil, err
	}
	return &paginationIterator{ResultIterator: iter, watchCh: watch.WatchCh()}, nil
}

// paginationIterator preserves the original index watch and stops a seek at the
// end of its requested range, rather than scanning into adjacent namespaces.
type paginationIterator struct {
	memdb.ResultIterator
	watchCh <-chan struct{}
	within  func(any) bool
	done    bool
}

func (i *paginationIterator) WatchCh() <-chan struct{} { return i.watchCh }

func (i *paginationIterator) Next() any {
	if i.done {
		return nil
	}
	raw := i.ResultIterator.Next()
	if raw == nil || (i.within != nil && !i.within(raw)) {
		i.done = true
		return nil
	}
	return raw
}
