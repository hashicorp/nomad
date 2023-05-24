// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestNodePools_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodePools := c.NodePools()

	testCases := []struct {
		name     string
		q        *QueryOptions
		expected []string
	}{
		{
			name: "list all",
			q:    nil,
			expected: []string{
				NodePoolAll,
				NodePoolDefault,
			},
		},
		{
			name: "with query param",
			q: &QueryOptions{
				PerPage: 1,
			},
			expected: []string{NodePoolAll},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _, err := nodePools.List(tc.q)
			must.NoError(t, err)

			got := make([]string, len(resp))
			for i, pool := range resp {
				got[i] = pool.Name
			}
			must.SliceContainsAll(t, got, tc.expected)
		})
	}
}

func TestNodePools_PrefixList(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodePools := c.NodePools()

	// Create test node pool.
	dev1 := &NodePool{Name: "dev-1"}
	_, err := nodePools.Register(dev1, nil)
	must.NoError(t, err)

	testCases := []struct {
		name     string
		prefix   string
		q        *QueryOptions
		expected []string
	}{
		{
			name:   "prefix",
			prefix: "d",
			q:      nil,
			expected: []string{
				NodePoolDefault,
				dev1.Name,
			},
		},
		{
			name:   "with query param",
			prefix: "d",
			q: &QueryOptions{
				PerPage: 1,
			},
			expected: []string{NodePoolDefault},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _, err := nodePools.PrefixList(tc.prefix, tc.q)
			must.NoError(t, err)

			got := make([]string, len(resp))
			for i, pool := range resp {
				got[i] = pool.Name
			}
			must.SliceContainsAll(t, got, tc.expected)
		})
	}
}

func TestNodePools_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodePools := c.NodePools()

	// Retrieve default node pool.
	pool, _, err := nodePools.Info(NodePoolDefault, nil)
	must.NoError(t, err)
	must.Eq(t, NodePoolDefault, pool.Name)
}

func TestNodePools_Register(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodePools := c.NodePools()

	// Create test node pool.
	dev1 := &NodePool{Name: "dev-1"}
	_, err := nodePools.Register(dev1, nil)
	must.NoError(t, err)

	// Verify node pool was persisted.
	got, _, err := nodePools.Info(dev1.Name, nil)
	must.NoError(t, err)
	must.Eq(t, dev1.Name, got.Name)

	// Update test node pool.
	dev1.Description = "test"
	_, err = nodePools.Register(dev1, nil)
	must.NoError(t, err)

	// Verify node pool was updated.
	got, _, err = nodePools.Info(dev1.Name, nil)
	must.NoError(t, err)
	must.Eq(t, dev1.Name, got.Name)
	must.Eq(t, dev1.Description, got.Description)
}

func TestNodePools_Delete(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodePools := c.NodePools()

	// Create test node pool.
	dev1 := &NodePool{Name: "dev-1"}
	_, err := nodePools.Register(dev1, nil)
	must.NoError(t, err)

	// Verify node pool was persisted.
	got, _, err := nodePools.Info(dev1.Name, nil)
	must.NoError(t, err)
	must.Eq(t, dev1.Name, got.Name)

	// Delete test node pool.
	_, err = nodePools.Delete(dev1.Name, nil)
	must.NoError(t, err)

	// Verify node pool is gone.
	got, _, err = nodePools.Info(dev1.Name, nil)
	must.ErrorContains(t, err, "not found")
}
