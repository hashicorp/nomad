// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_NodePools(t *testing.T) {
	ci.Parallel(t)

	// Create test node pools.
	state := testStateStore(t)
	pools := make([]*structs.NodePool, 10)
	for i := 0; i < 10; i++ {
		pools[i] = mock.NodePool()
	}
	must.NoError(t, state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools))

	// Create a watchset to test that getters don't cause it to fire.
	ws := memdb.NewWatchSet()
	iter, err := state.NodePools(ws, SortDefault)
	must.NoError(t, err)

	// Verify all pools are returned.
	foundBuiltIn := map[string]bool{
		structs.NodePoolAll:     false,
		structs.NodePoolDefault: false,
	}
	got := make([]*structs.NodePool, 0, 10)

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		pool := raw.(*structs.NodePool)

		if pool.IsBuiltIn() {
			must.False(t, foundBuiltIn[pool.Name])
			foundBuiltIn[pool.Name] = true
			continue
		}

		got = append(got, pool)
	}

	must.SliceContainsAll(t, got, pools)
	must.False(t, watchFired(ws))
	for k, v := range foundBuiltIn {
		must.True(t, v, must.Sprintf("built-in pool %q not found", k))
	}
}

func TestStateStore_NodePools_Ordering(t *testing.T) {
	ci.Parallel(t)

	// Create test node pools with stable sortable names.
	state := testStateStore(t)
	pools := make([]*structs.NodePool, 10)
	for i := 0; i < 5; i++ {
		pool := mock.NodePool()
		pool.Name = fmt.Sprintf("%02d", i+1)
		pools[i] = pool
	}
	must.NoError(t, state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools))

	testCases := []struct {
		name     string
		order    SortOption
		expected []string
	}{
		{
			name:     "default order",
			order:    SortDefault,
			expected: []string{"01", "02", "03", "04", "05"},
		},
		{
			name:     "reverse order",
			order:    SortReverse,
			expected: []string{"05", "04", "03", "02", "01"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			iter, err := state.NodePools(ws, tc.order)
			must.NoError(t, err)

			var got []string
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				pool := raw.(*structs.NodePool)
				if pool.IsBuiltIn() {
					continue
				}

				got = append(got, pool.Name)
			}

			must.Eq(t, got, tc.expected)
		})
	}
}

func TestStateStore_NodePool_ByName(t *testing.T) {
	ci.Parallel(t)

	// Create test node pools.
	state := testStateStore(t)
	pools := make([]*structs.NodePool, 10)
	for i := 0; i < 10; i++ {
		pools[i] = mock.NodePool()
	}
	must.NoError(t, state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools))

	testCases := []struct {
		name     string
		pool     string
		expected *structs.NodePool
	}{
		{
			name:     "find a pool",
			pool:     pools[3].Name,
			expected: pools[3],
		},
		{
			name: "find built-in pool all",
			pool: structs.NodePoolAll,
			expected: &structs.NodePool{
				Name:        structs.NodePoolAll,
				Description: structs.NodePoolAllDescription,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "find built-in pool default",
			pool: structs.NodePoolDefault,
			expected: &structs.NodePool{
				Name:        structs.NodePoolDefault,
				Description: structs.NodePoolDefaultDescription,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name:     "pool not found",
			pool:     "no-pool",
			expected: nil,
		},
		{
			name:     "must be exact match",
			pool:     pools[2].Name[:4],
			expected: nil,
		},
		{
			name:     "empty search",
			pool:     "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			got, err := state.NodePoolByName(ws, tc.pool)

			must.NoError(t, err)
			must.Eq(t, tc.expected, got)
			must.False(t, watchFired(ws))
		})
	}
}

func TestStateStore_NodePool_ByNamePrefix(t *testing.T) {
	ci.Parallel(t)

	// Create test node pools.
	state := testStateStore(t)
	existingPools := []*structs.NodePool{
		{Name: "prod-1"},
		{Name: "prod-2"},
		{Name: "prod-3"},
		{Name: "dev-1"},
		{Name: "dev-2"},
		{Name: "qa"},
	}
	err := state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, existingPools)
	must.NoError(t, err)

	testCases := []struct {
		name     string
		prefix   string
		expected []string
		order    SortOption
	}{
		{
			name:     "multiple prefix match",
			prefix:   "prod",
			order:    SortDefault,
			expected: []string{"prod-1", "prod-2", "prod-3"},
		},
		{
			name:     "single prefix match",
			prefix:   "qa",
			order:    SortDefault,
			expected: []string{"qa"},
		},
		{
			name:     "no match",
			prefix:   "nope",
			order:    SortDefault,
			expected: []string{},
		},
		{
			name:   "empty prefix",
			prefix: "",
			order:  SortDefault,
			expected: []string{
				"all",
				"default",
				"prod-1",
				"prod-2",
				"prod-3",
				"dev-1",
				"dev-2",
				"qa",
			},
		},
		{
			name:     "reverse order",
			prefix:   "prod",
			order:    SortReverse,
			expected: []string{"prod-3", "prod-2", "prod-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			iter, err := state.NodePoolsByNamePrefix(ws, tc.prefix, tc.order)
			must.NoError(t, err)

			got := []string{}
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				got = append(got, raw.(*structs.NodePool).Name)
			}
			must.SliceContainsAll(t, tc.expected, got)
		})
	}
}

func TestStateStore_NodePool_Upsert(t *testing.T) {
	ci.Parallel(t)

	existingPools := make([]*structs.NodePool, 10)
	for i := 0; i < 10; i++ {
		existingPools[i] = mock.NodePool()
	}

	testCases := []struct {
		name        string
		input       []*structs.NodePool
		expectedErr string
	}{
		{
			name: "add single pool",
			input: []*structs.NodePool{
				mock.NodePool(),
			},
		},
		{
			name: "add multiple pools",
			input: []*structs.NodePool{
				mock.NodePool(),
				mock.NodePool(),
				mock.NodePool(),
			},
		},
		{
			name: "update existing pools",
			input: []*structs.NodePool{
				{
					Name:        existingPools[0].Name,
					Description: "updated",
					Meta: map[string]string{
						"updated": "true",
					},
					SchedulerConfiguration: &structs.NodePoolSchedulerConfiguration{
						SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
					},
				},
				{
					Name:        existingPools[1].Name,
					Description: "use global scheduler config",
				},
			},
		},
		{
			name: "update with nil",
			input: []*structs.NodePool{
				nil,
			},
		},
		{
			name: "empty name",
			input: []*structs.NodePool{
				{
					Name: "",
				},
			},
			expectedErr: "missing primary index",
		},
		{
			name: "update bulit-in pool all",
			input: []*structs.NodePool{
				{
					Name:        structs.NodePoolAll,
					Description: "changed",
				},
			},
			expectedErr: "not allowed",
		},
		{
			name: "update built-in pool default",
			input: []*structs.NodePool{
				{
					Name:        structs.NodePoolDefault,
					Description: "changed",
				},
			},
			expectedErr: "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test pools.
			state := testStateStore(t)
			must.NoError(t, state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, existingPools))

			// Update pools from test case.
			err := state.UpsertNodePools(structs.MsgTypeTestSetup, 1001, tc.input)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				ws := memdb.NewWatchSet()
				for _, pool := range tc.input {
					if pool == nil {
						continue
					}

					got, err := state.NodePoolByName(ws, pool.Name)
					must.NoError(t, err)
					must.Eq(t, pool, got)
				}
			}
		})
	}
}

func TestStateStore_NodePool_Delete(t *testing.T) {
	ci.Parallel(t)

	pools := make([]*structs.NodePool, 10)
	for i := 0; i < 10; i++ {
		pools[i] = mock.NodePool()
	}

	testCases := []struct {
		name        string
		del         []string
		expectedErr string
	}{
		{
			name: "delete one",
			del:  []string{pools[0].Name},
		},
		{
			name: "delete multiple",
			del:  []string{pools[0].Name, pools[3].Name},
		},
		{
			name:        "delete non-existing",
			del:         []string{"nope"},
			expectedErr: "not found",
		},
		{
			name:        "delete is atomic",
			del:         []string{pools[0].Name, "nope"},
			expectedErr: "not found",
		},
		{
			name:        "delete built-in pool all",
			del:         []string{structs.NodePoolAll},
			expectedErr: "not allowed",
		},
		{
			name:        "delete built-in pool default",
			del:         []string{structs.NodePoolDefault},
			expectedErr: "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := testStateStore(t)
			must.NoError(t, state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools))

			err := state.DeleteNodePools(structs.MsgTypeTestSetup, 1001, tc.del)
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)

				// Make sure delete is atomic and nothing is removed if an
				// error happens.
				for _, p := range pools {
					got, err := state.NodePoolByName(nil, p.Name)
					must.NoError(t, err)
					must.Eq(t, p, got)
				}
			} else {
				must.NoError(t, err)

				// Check that the node pools is deleted.
				for _, p := range tc.del {
					got, err := state.NodePoolByName(nil, p)
					must.NoError(t, err)
					must.Nil(t, got)
				}
			}
		})
	}
}

func TestStateStore_NodePool_Restore(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	pool := mock.NodePool()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.NodePoolRestore(pool)
	must.NoError(t, err)

	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.NodePoolByName(ws, pool.Name)
	must.NoError(t, err)
	must.Eq(t, out, pool)
}
