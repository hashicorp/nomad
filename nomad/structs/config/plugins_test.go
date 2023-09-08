// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"sort"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestPluginConfig_Merge(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	a := &PluginConfig{
		Name: "foo",
		Args: []string{"bar"},
		Config: map[string]interface{}{
			"baz": true,
		},
	}

	e1 := &PluginConfig{
		Name: "replaced",
		Args: []string{"bar"},
		Config: map[string]interface{}{
			"baz": true,
		},
	}
	o1 := a.Merge(&PluginConfig{Name: "replaced"})
	require.Equal(e1, o1)

	e2 := &PluginConfig{
		Name: "foo",
		Args: []string{"replaced", "another"},
		Config: map[string]interface{}{
			"baz": true,
		},
	}
	o2 := a.Merge(&PluginConfig{
		Args: []string{"replaced", "another"},
	})
	require.Equal(e2, o2)

	e3 := &PluginConfig{
		Name: "foo",
		Args: []string{"bar"},
		Config: map[string]interface{}{
			"replaced": 1,
		},
	}
	o3 := a.Merge(&PluginConfig{
		Config: map[string]interface{}{
			"replaced": 1,
		},
	})
	require.Equal(e3, o3)
}

func TestPluginConfigSet_Merge(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	a := &PluginConfig{
		Name: "a",
		Args: []string{"a1"},
		Config: map[string]interface{}{
			"a1": true,
		},
	}
	b1 := &PluginConfig{
		Name: "b",
		Args: []string{"b1"},
		Config: map[string]interface{}{
			"b1": true,
		},
	}
	b2 := &PluginConfig{
		Name: "b",
		Args: []string{"b2"},
		Config: map[string]interface{}{
			"b2": true,
		},
	}
	c := &PluginConfig{
		Name: "c",
		Args: []string{"c"},
		Config: map[string]interface{}{
			"c1": true,
		},
	}

	s1 := []*PluginConfig{a, b1}
	s2 := []*PluginConfig{b2, c}

	out := PluginConfigSetMerge(s1, s2)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	expected := []*PluginConfig{a, b2, c}
	require.EqualValues(expected, out)
}
