// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_AllocsFrom(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	txn := store.db.WriteTxn(100)
	var allocs []*structs.Allocation
	for _, namespace := range []string{"a", "default", "default-extra", "z"} {
		for _, index := range []uint64{2, 10, 10, 100} {
			alloc := mock.Alloc()
			alloc.ID = fmt.Sprintf("%08x-0000-0000-0000-000000000000", len(allocs)+1)
			alloc.Namespace = namespace
			alloc.CreateIndex = index
			if len(allocs)%2 == 0 {
				alloc.TaskGroup = "other"
			}
			allocs = append(allocs, alloc)
			must.NoError(t, txn.Insert("allocs", alloc))
		}
	}
	must.NoError(t, txn.Commit())

	cursor := fmt.Sprintf("10.%s", allocs[6].ID)
	const webFilter = `TaskGroup == "web"`
	for _, tc := range []struct {
		name      string
		namespace string
		token     string
		reverse   bool
		filter    string
	}{
		{
			name:      "first page",
			namespace: "default",
		},
		{
			name:      "first page reverse",
			namespace: "default",
			reverse:   true,
		},
		{
			name:      "cursor",
			namespace: "default",
			token:     cursor,
		},
		{
			name:      "cursor reverse",
			namespace: "default",
			token:     cursor,
			reverse:   true,
		},
		{
			name:      "filtered cursor",
			namespace: "default",
			token:     cursor,
			filter:    webFilter,
		},
		{
			name:      "filtered cursor reverse",
			namespace: "default",
			token:     cursor,
			reverse:   true,
			filter:    webFilter,
		},
		{
			name:      "missing namespace",
			namespace: "missing",
			token:     cursor,
		},
		{
			name:      "missing namespace reverse",
			namespace: "missing",
			token:     cursor,
			reverse:   true,
		},
		{
			name:      "all namespaces first page",
			namespace: "*",
		},
		{
			name:      "all namespaces first page reverse",
			namespace: "*",
			reverse:   true,
		},
		{
			name:      "all namespaces cursor",
			namespace: "*",
			token:     cursor,
		},
		{
			name:      "all namespaces cursor reverse",
			namespace: "*",
			token:     cursor,
			reverse:   true,
		},
		{
			name:      "all namespaces filtered cursor",
			namespace: "*",
			token:     cursor,
			filter:    webFilter,
		},
		{
			name:      "all namespaces filtered cursor reverse",
			namespace: "*",
			token:     cursor,
			reverse:   true,
			filter:    webFilter,
		},
		{
			name:      "missing index",
			namespace: "default",
			token:     "9." + allocs[6].ID,
		},
		{
			name:      "missing index reverse",
			namespace: "default",
			token:     "9." + allocs[6].ID,
			reverse:   true,
		},
		{
			name:      "missing ID",
			namespace: "default",
			token:     "10.00000006-0000-0000-0000-000000000001",
		},
		{
			name:      "missing ID reverse",
			namespace: "default",
			token:     "10.00000006-0000-0000-0000-000000000001",
			reverse:   true,
		},
		{
			name:      "before first",
			namespace: "default",
			token:     "0.",
		},
		{
			name:      "before first reverse",
			namespace: "default",
			token:     "0.",
			reverse:   true,
		},
		{
			name:      "after last",
			namespace: "default",
			token:     "101.",
		},
		{
			name:      "after last reverse",
			namespace: "default",
			token:     "101.",
			reverse:   true,
		},
		{
			name:      "missing separator",
			namespace: "default",
			token:     "invalid",
		},
		{
			name:      "missing separator reverse",
			namespace: "default",
			token:     "invalid",
			reverse:   true,
		},
		{
			name:      "invalid index",
			namespace: "default",
			token:     "invalid.id",
		},
		{
			name:      "invalid index reverse",
			namespace: "default",
			token:     "invalid.id",
			reverse:   true,
		},
		{
			name:      "negative index",
			namespace: "default",
			token:     "-1.id",
		},
		{
			name:      "negative index reverse",
			namespace: "default",
			token:     "-1.id",
			reverse:   true,
		},
		{
			name:      "empty ID",
			namespace: "default",
			token:     "10.",
		},
		{
			name:      "empty ID reverse",
			namespace: "default",
			token:     "10.",
			reverse:   true,
		},
		{
			name:      "null in ID",
			namespace: "default",
			token:     "10.id\x00",
		},
		{
			name:      "null in ID reverse",
			namespace: "default",
			token:     "10.id\x00",
			reverse:   true,
		},
		{
			name:      "index overflow",
			namespace: "default",
			token:     "99999999999999999999999999.id",
		},
		{
			name:      "index overflow reverse",
			namespace: "default",
			token:     "99999999999999999999999999.id",
			reverse:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			var original memdb.ResultIterator
			var err error
			if tc.namespace == structs.AllNamespacesSentinel {
				original, err = store.Allocs(ws, SortOption(tc.reverse))
			} else {
				original, err = store.AllocsByNamespaceOrdered(ws, tc.namespace, SortOption(tc.reverse))
			}
			must.NoError(t, err)
			seek, err := store.AllocsFrom(ws, tc.namespace, tc.token, SortOption(tc.reverse))
			must.NoError(t, err)
			must.NotNil(t, seek.WatchCh())

			opts := structs.QueryOptions{NextToken: tc.token, Reverse: tc.reverse, PerPage: 2, Filter: tc.filter}
			// Also exercise ACL-style selection after the seek.
			selector := func(a *structs.Allocation) bool { return a.Namespace != "a" }
			stub := func(a *structs.Allocation) (string, error) { return a.ID, nil }
			baseline, err := paginator.NewPaginator(original, opts, selector,
				paginator.CreateIndexAndIDTokenizer[*structs.Allocation](tc.token), stub)
			must.NoError(t, err)
			optimized, err := paginator.NewPaginator(seek, opts, selector,
				paginator.CreateIndexAndIDTokenizer[*structs.Allocation](tc.token), stub)
			must.NoError(t, err)
			want, wantNext, err := baseline.Page()
			must.NoError(t, err)
			got, gotNext, err := optimized.Page()
			must.NoError(t, err)
			must.Eq(t, want, got)
			must.Eq(t, wantNext, gotNext)
		})
	}

	// Assert that seeking actually positions the underlying iterator, and
	// stops at namespace boundaries in both directions.
	for _, tc := range []struct {
		reverse bool
		want    []*structs.Allocation
	}{
		{
			reverse: false,
			want:    allocs[6:8],
		},
		{
			reverse: true,
			want:    []*structs.Allocation{allocs[6], allocs[5], allocs[4]},
		},
	} {
		ws := memdb.NewWatchSet()
		iter, err := store.AllocsFrom(ws, "default", cursor, SortOption(tc.reverse))
		must.NoError(t, err)
		for _, want := range tc.want {
			must.Eq(t, want, iter.Next().(*structs.Allocation))
		}
		must.Nil(t, iter.Next())
		must.Nil(t, iter.Next())
	}

	// A deleted cursor must still seek to its successor/predecessor.
	txn = store.db.WriteTxn(101)
	must.NoError(t, txn.Delete("allocs", allocs[6]))
	must.NoError(t, txn.Commit())
	for _, tc := range []struct {
		reverse bool
		want    *structs.Allocation
	}{
		{
			reverse: false,
			want:    allocs[7],
		},
		{
			reverse: true,
			want:    allocs[5],
		},
	} {
		iter, err := store.AllocsFrom(memdb.NewWatchSet(), "default", cursor, SortOption(tc.reverse))
		must.NoError(t, err)
		must.Eq(t, tc.want, iter.Next().(*structs.Allocation))
	}
}

func TestStateStore_AllocsFrom_Watch(t *testing.T) {
	ci.Parallel(t)
	for _, namespace := range []string{"default", structs.AllNamespacesSentinel} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", namespace, reverse), func(t *testing.T) {
				store := testStateStore(t)
				ws := memdb.NewWatchSet()
				iter, err := store.AllocsFrom(ws, namespace, "10.id", SortOption(reverse))
				must.NoError(t, err)
				must.Nil(t, iter.Next())
				must.NotNil(t, iter.WatchCh())
				must.False(t, watchFired(ws))
				txn := store.db.WriteTxn(100)
				must.NoError(t, txn.Insert("allocs", mock.Alloc()))
				must.NoError(t, txn.Commit())
				must.True(t, watchFired(ws))
			})
		}
	}
}

func TestStateStore_NodesFrom(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	var nodes []*structs.Node
	for i := range 4 {
		node := mock.Node()
		node.ID = fmt.Sprintf("%08x-abcd-0000-0000-000000000000", i*2+1)
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), node))
		nodes = append(nodes, node)
	}
	for _, token := range []string{"", nodes[1].ID, "00000004-abcd-0000-0000-000000000000", "invalid", strings.ToUpper(nodes[1].ID)} {
		t.Run(token, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			baseline, err := store.Nodes(ws)
			must.NoError(t, err)
			seek, err := store.NodesFrom(ws, token)
			must.NoError(t, err)
			must.NotNil(t, seek.WatchCh())
			opts := structs.QueryOptions{NextToken: token, PerPage: 2}
			stub := func(n *structs.Node) (string, error) { return n.ID, nil }
			original, err := paginator.NewPaginator(baseline, opts, nil, paginator.IDTokenizer[*structs.Node](token), stub)
			must.NoError(t, err)
			optimized, err := paginator.NewPaginator(seek, opts, nil, paginator.IDTokenizer[*structs.Node](token), stub)
			must.NoError(t, err)
			want, wantNext, err := original.Page()
			must.NoError(t, err)
			got, gotNext, err := optimized.Page()
			must.NoError(t, err)
			must.Eq(t, want, got)
			must.Eq(t, wantNext, gotNext)
		})
	}

	must.NoError(t, store.DeleteNode(structs.MsgTypeTestSetup, 200, []string{nodes[1].ID}))
	ws := memdb.NewWatchSet()
	iter, err := store.NodesFrom(ws, nodes[1].ID)
	must.NoError(t, err)
	must.Eq(t, nodes[2].ID, iter.Next().(*structs.Node).ID)
	must.False(t, watchFired(ws))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 201, mock.Node()))
	must.True(t, watchFired(ws))
}
