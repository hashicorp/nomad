// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_OpaqueMapsEqual(t *testing.T) {
	ci.Parallel(t)

	type public struct {
		F int
	}

	type private struct {
		g int
	}

	type mix struct {
		F int
		g int
	}

	cases := []struct {
		name string
		a, b map[string]any
		exp  bool
	}{{
		name: "both nil",
		a:    nil,
		b:    nil,
		exp:  true,
	}, {
		name: "empty and nil",
		a:    nil,
		b:    make(map[string]any),
		exp:  true,
	}, {
		name: "same strings",
		a:    map[string]any{"a": "A"},
		b:    map[string]any{"a": "A"},
		exp:  true,
	}, {
		name: "same public struct",
		a:    map[string]any{"a": &public{F: 42}},
		b:    map[string]any{"a": &public{F: 42}},
		exp:  true,
	}, {
		name: "different public struct",
		a:    map[string]any{"a": &public{F: 42}},
		b:    map[string]any{"a": &public{F: 10}},
		exp:  false,
	}, {
		name: "different private struct",
		a:    map[string]any{"a": &private{g: 42}},
		b:    map[string]any{"a": &private{g: 10}},
		exp:  true, // private fields not compared
	}, {
		name: "mix same public different private",
		a:    map[string]any{"a": &mix{F: 42, g: 1}},
		b:    map[string]any{"a": &mix{F: 42, g: 2}},
		exp:  true, // private fields not compared
	}, {
		name: "mix different public same private",
		a:    map[string]any{"a": &mix{F: 42, g: 1}},
		b:    map[string]any{"a": &mix{F: 10, g: 1}},
		exp:  false,
	}, {
		name: "nil vs empty slice values",
		a:    map[string]any{"key": []string(nil)},
		b:    map[string]any{"key": make([]string, 0)},
		exp:  true,
	}, {
		name: "nil vs empty map values",
		a:    map[string]any{"key": map[int]int(nil)},
		b:    map[string]any{"key": make(map[int]int, 0)},
		exp:  true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := OpaqueMapsEqual(tc.a, tc.b)
			must.Eq(t, tc.exp, result)
		})
	}
}
