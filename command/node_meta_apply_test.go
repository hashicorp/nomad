// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
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
				"key1": pointer.Of("val1"),
				"key2": pointer.Of("val2"),
				"key3": pointer.Of(""),
			},
		},
		{
			name:  "EmptyNoEquals",
			input: []string{"key1=val1", "key2=val2", "key4"},
			exp: map[string]*string{
				"key1": pointer.Of("val1"),
				"key2": pointer.Of("val2"),
				"key4": pointer.Of(""),
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
				"": pointer.Of(""),
			},
		},
		{
			name:  "WeirdArgs",
			input: []string{"=", "foo==bar"},
			exp: map[string]*string{
				"":    pointer.Of(""),
				"foo": pointer.Of("=bar"),
			},
		},
		{
			name:  "WeirderArgs",
			input: []string{"=foo=bar", "\x00=\x01"},
			exp: map[string]*string{
				"":     pointer.Of("foo=bar"),
				"\x00": pointer.Of("\x01"),
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
				"foo": pointer.Of("bar"),
			},
			exp: map[string]*string{
				"foo": pointer.Of("bar"),
			},
		},
		{
			name:  "Empty",
			unset: "",
			meta: map[string]*string{
				"foo": pointer.Of("bar"),
			},
			exp: map[string]*string{
				"foo": pointer.Of("bar"),
			},
		},
		{
			name:  "UnsetNew",
			unset: "unset",
			meta: map[string]*string{
				"foo": pointer.Of("bar"),
			},
			exp: map[string]*string{
				"foo":   pointer.Of("bar"),
				"unset": nil,
			},
		},
		{
			name:  "UnsetExisting",
			unset: "foo",
			meta: map[string]*string{
				"foo": pointer.Of("bar"),
			},
			exp: map[string]*string{
				"foo": nil,
			},
		},
		{
			name:  "UnsetBoth",
			unset: ",foo,unset,",
			meta: map[string]*string{
				"foo": pointer.Of("bar"),
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
