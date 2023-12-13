// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// TestAddNestedKey_Ok asserts test cases that succeed when passed to
// addNestedKey.
func TestAddNestedKey_Ok(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		// M will be initialized if unset
		M map[string]interface{}
		K string
		// Value is always "x"
		Result map[string]interface{}
	}{
		{
			K: "foo",
			Result: map[string]interface{}{
				"foo": "x",
			},
		},
		{
			K: "foo.bar",
			Result: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "x",
				},
			},
		},
		{
			K: "foo.bar.quux",
			Result: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"quux": "x",
					},
				},
			},
		},
		{
			K: "a.b.c",
			Result: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "x",
					},
				},
			},
		},
		{
			// Nested object b should take precedence over values
			M: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "c",
					},
				},
			},
			K: "a.b",
			Result: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "c",
					},
				},
			},
		},
		{
			M: map[string]interface{}{
				"a": map[string]interface{}{
					"x": "x",
				},
				"z": "z",
			},
			K: "a.b.c",
			Result: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "x",
					},
					"x": "x",
				},
				"z": "z",
			},
		},
		{
			M: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"a":    "z",
						"quux": "z",
					},
				},
			},
			K: "foo.bar.quux",
			Result: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"a":    "z",
						"quux": "x",
					},
				},
			},
		},
		{
			M: map[string]interface{}{
				"foo":  "1",
				"bar":  "2",
				"quux": "3",
			},
			K: "a.bbbbbb.c",
			Result: map[string]interface{}{
				"foo":  "1",
				"bar":  "2",
				"quux": "3",
				"a": map[string]interface{}{
					"bbbbbb": map[string]interface{}{
						"c": "x",
					},
				},
			},
		},
		// Regardless of whether attr.driver.qemu = "1" is added first
		// or second, attr.driver.qemu.version = "..." should take
		// precedence (nested maps take precedence over values)
		{
			M: map[string]interface{}{
				"attr": map[string]interface{}{
					"driver": map[string]interface{}{
						"qemu": "1",
					},
				},
			},
			K: "attr.driver.qemu.version",
			Result: map[string]interface{}{
				"attr": map[string]interface{}{
					"driver": map[string]interface{}{
						"qemu": map[string]interface{}{
							"version": "x",
						},
					},
				},
			},
		},
		{
			M: map[string]interface{}{
				"attr": map[string]interface{}{
					"driver": map[string]interface{}{
						"qemu": map[string]interface{}{
							"version": "1.2.3",
						},
					},
				},
			},
			K: "attr.driver.qemu",
			Result: map[string]interface{}{
				"attr": map[string]interface{}{
					"driver": map[string]interface{}{
						"qemu": map[string]interface{}{
							"version": "1.2.3",
						},
					},
				},
			},
		},
		{
			M: map[string]interface{}{
				"a": "a",
			},
			K: "a.b",
			Result: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "x",
				},
			},
		},
		{
			M: map[string]interface{}{
				"a": "a",
				"foo": map[string]interface{}{
					"b":   "b",
					"bar": "quux",
				},
				"c": map[string]interface{}{},
			},
			K: "foo.bar.quux",
			Result: map[string]interface{}{
				"a": "a",
				"foo": map[string]interface{}{
					"b": "b",
					"bar": map[string]interface{}{
						"quux": "x",
					},
				},
				"c": map[string]interface{}{},
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		name := tc.K
		if len(tc.M) > 0 {
			name = fmt.Sprintf("%s-%d", name, len(tc.M))
		}
		t.Run(name, func(t *testing.T) {
			ci.Parallel(t)
			if tc.M == nil {
				tc.M = map[string]interface{}{}
			}
			require.NoError(t, addNestedKey(tc.M, tc.K, "x"))
			require.Equal(t, tc.Result, tc.M)
		})
	}
}

// TestAddNestedKey_Bad asserts test cases return an error when passed to
// addNestedKey.
func TestAddNestedKey_Bad(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		// M will be initialized if unset
		M func() map[string]interface{}
		K string
		// Value is always "x"
		// Result is compared by Error() string equality
		Result error
	}{
		{
			K:      ".",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      ".foo",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo.",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      ".a.",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo..bar",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo...bar",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo.bar..quux",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo..bar.quux",
			Result: ErrInvalidObjectPath,
		},
		{
			K:      "foo.bar.quux.",
			Result: ErrInvalidObjectPath,
		},
		{
			M: func() map[string]interface{} {
				return map[string]interface{}{
					"a": "a",
					"foo": map[string]interface{}{
						"b": "b",
						"bar": map[string]interface{}{
							"c": "c",
						},
					},
				}
			},
			K:      "foo.bar.quux.",
			Result: ErrInvalidObjectPath,
		},
		{
			M: func() map[string]interface{} {
				return map[string]interface{}{
					"a": "a",
					"foo": map[string]interface{}{
						"b": "b",
						"bar": map[string]interface{}{
							"c": "c",
						},
					},
				}
			},
			K:      "foo.bar..quux",
			Result: ErrInvalidObjectPath,
		},
		{
			M: func() map[string]interface{} {
				return map[string]interface{}{
					"a": "a",
					"foo": map[string]interface{}{
						"b": "b",
						"bar": map[string]interface{}{
							"c": "c",
						},
					},
				}
			},
			K:      "foo.bar..quux",
			Result: ErrInvalidObjectPath,
		},
	}

	for i := range cases {
		tc := cases[i]
		name := tc.K
		if tc.M != nil {
			name += "-cleanup"
		}
		t.Run(name, func(t *testing.T) {
			ci.Parallel(t)

			// Copy original M value to ensure it doesn't get altered
			if tc.M == nil {
				tc.M = func() map[string]interface{} {
					return map[string]interface{}{}
				}
			}

			// Call func and assert error
			m := tc.M()
			err := addNestedKey(m, tc.K, "x")
			require.EqualError(t, err, tc.Result.Error())

			// Ensure M wasn't altered
			require.Equal(t, tc.M(), m)
		})
	}
}

func TestCtyify_Ok(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name string
		In   map[string]interface{}
		Out  map[string]cty.Value
	}{
		{
			Name: "OneVal",
			In: map[string]interface{}{
				"a": "b",
			},
			Out: map[string]cty.Value{
				"a": cty.StringVal("b"),
			},
		},
		{
			Name: "MultiVal",
			In: map[string]interface{}{
				"a":   "b",
				"foo": "bar",
			},
			Out: map[string]cty.Value{
				"a":   cty.StringVal("b"),
				"foo": cty.StringVal("bar"),
			},
		},
		{
			Name: "NestedVals",
			In: map[string]interface{}{
				"a": "b",
				"foo": map[string]interface{}{
					"c": "d",
					"bar": map[string]interface{}{
						"quux": "z",
					},
				},
				"123": map[string]interface{}{
					"bar": map[string]interface{}{
						"456": "789",
					},
				},
			},
			Out: map[string]cty.Value{
				"a": cty.StringVal("b"),
				"foo": cty.ObjectVal(map[string]cty.Value{
					"c": cty.StringVal("d"),
					"bar": cty.ObjectVal(map[string]cty.Value{
						"quux": cty.StringVal("z"),
					}),
				}),
				"123": cty.ObjectVal(map[string]cty.Value{
					"bar": cty.ObjectVal(map[string]cty.Value{
						"456": cty.StringVal("789"),
					}),
				}),
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			ci.Parallel(t)

			// ctiyif and check for errors
			result, err := ctyify(tc.In)
			require.NoError(t, err)

			// convert results to ObjectVals and compare with RawEquals
			resultObj := cty.ObjectVal(result)
			OutObj := cty.ObjectVal(tc.Out)
			require.True(t, OutObj.RawEquals(resultObj))
		})
	}
}

func TestCtyify_Bad(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name string
		In   map[string]interface{}
		Out  map[string]cty.Value
	}{
		{
			Name: "NonStringVal",
			In: map[string]interface{}{
				"a": 1,
			},
		},
		{
			Name: "NestedNonString",
			In: map[string]interface{}{
				"foo": map[string]interface{}{
					"c": 1,
				},
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			ci.Parallel(t)

			// ctiyif and check for errors
			result, err := ctyify(tc.In)
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}
