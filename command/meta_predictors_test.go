// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestMeta_NodePoolPredictor(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register some test node pools.
	dev1 := &api.NodePool{Name: "dev-1"}
	_, err := client.NodePools().Register(dev1, nil)
	must.NoError(t, err)

	dev2 := &api.NodePool{Name: "dev-2"}
	_, err = client.NodePools().Register(dev2, nil)
	must.NoError(t, err)

	prod := &api.NodePool{Name: "prod"}
	_, err = client.NodePools().Register(prod, nil)
	must.NoError(t, err)

	testCases := []struct {
		name     string
		args     complete.Args
		filter   *set.Set[string]
		expected []string
	}{
		{
			name: "find with prefix",
			args: complete.Args{
				Last: "de",
			},
			expected: []string{"default", "dev-1", "dev-2"},
		},
		{
			name: "filter",
			args: complete.Args{
				Last: "de",
			},
			filter:   set.From([]string{"default"}),
			expected: []string{"dev-1", "dev-2"},
		},
		{
			name: "find all",
			args: complete.Args{
				Last: "",
			},
			expected: []string{"all", "default", "dev-1", "dev-2", "prod"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Meta{flagAddress: url}
			got := m.NodePoolPredictor(tc.filter).Predict(tc.args)
			must.SliceContainsAll(t, tc.expected, got)
		})
	}
}
