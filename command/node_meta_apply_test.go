// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestNodeMeta_parseMapFromArgs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		input []string
		exp   map[string]*string
	}{
		{
			name:  "EmptyEquals",
			input: []string{"key1=val1", "key2=val2", "key3="},
			exp: map[string]*string{
				"key1": new("val1"),
				"key2": new("val2"),
				"key3": new(""),
			},
		},
		{
			name:  "EmptyNoEquals",
			input: []string{"key1=val1", "key2=val2", "key4"},
			exp: map[string]*string{
				"key1": new("val1"),
				"key2": new("val2"),
				"key4": new(""),
			},
		},
		{
			name:  "Nil",
			input: nil,
			exp:   map[string]*string{},
		},
		{
			name:  "Empty",
			input: []string{},
			exp:   map[string]*string{},
		},
		{
			name:  "EmptyArg",
			input: []string{""},
			exp: map[string]*string{
				"": new(""),
			},
		},
		{
			name:  "WeirdArgs",
			input: []string{"=", "foo==bar"},
			exp: map[string]*string{
				"":    new(""),
				"foo": new("=bar"),
			},
		},
		{
			name:  "WeirderArgs",
			input: []string{"=foo=bar", "\x00=\x01"},
			exp: map[string]*string{
				"":     new("foo=bar"),
				"\x00": new("\x01"),
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			must.MapEq(t, tc.exp, parseMapFromArgs(tc.input))
		})
	}
}

func TestNodeMeta_applyNodeMetaUnset(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name  string
		unset string
		meta  map[string]*string
		exp   map[string]*string
	}{
		{
			name:  "CommaParty",
			unset: ",,,",
			meta: map[string]*string{
				"foo": new("bar"),
			},
			exp: map[string]*string{
				"foo": new("bar"),
			},
		},
		{
			name:  "Empty",
			unset: "",
			meta: map[string]*string{
				"foo": new("bar"),
			},
			exp: map[string]*string{
				"foo": new("bar"),
			},
		},
		{
			name:  "UnsetNew",
			unset: "unset",
			meta: map[string]*string{
				"foo": new("bar"),
			},
			exp: map[string]*string{
				"foo":   new("bar"),
				"unset": nil,
			},
		},
		{
			name:  "UnsetExisting",
			unset: "foo",
			meta: map[string]*string{
				"foo": new("bar"),
			},
			exp: map[string]*string{
				"foo": nil,
			},
		},
		{
			name:  "UnsetBoth",
			unset: ",foo,unset,",
			meta: map[string]*string{
				"foo": new("bar"),
			},
			exp: map[string]*string{
				"foo":   nil,
				"unset": nil,
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			applyNodeMetaUnset(tc.meta, tc.unset)
			must.MapEq(t, tc.exp, tc.meta)
		})
	}
}
