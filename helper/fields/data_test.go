// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fields

import (
	"reflect"
	"testing"
)

func TestFieldDataGet(t *testing.T) {
	cases := map[string]struct {
		Schema map[string]*FieldSchema
		Raw    map[string]interface{}
		Key    string
		Value  interface{}
	}{
		"string type, string value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeString},
			},
			map[string]interface{}{
				"foo": "bar",
			},
			"foo",
			"bar",
		},

		"string type, int value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeInt},
			},
			map[string]interface{}{
				"foo": 42,
			},
			"foo",
			42,
		},

		"string type, unset value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeString},
			},
			map[string]interface{}{},
			"foo",
			"",
		},

		"string type, unset value with default": {
			map[string]*FieldSchema{
				"foo": {
					Type:    TypeString,
					Default: "bar",
				},
			},
			map[string]interface{}{},
			"foo",
			"bar",
		},

		"int type, int value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeInt},
			},
			map[string]interface{}{
				"foo": 42,
			},
			"foo",
			42,
		},

		"bool type, bool value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeBool},
			},
			map[string]interface{}{
				"foo": false,
			},
			"foo",
			false,
		},

		"map type, map value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeMap},
			},
			map[string]interface{}{
				"foo": map[string]interface{}{
					"child": true,
				},
			},
			"foo",
			map[string]interface{}{
				"child": true,
			},
		},

		"array type, array value": {
			map[string]*FieldSchema{
				"foo": {Type: TypeArray},
			},
			map[string]interface{}{
				"foo": []interface{}{},
			},
			"foo",
			[]interface{}{},
		},
	}

	for name, tc := range cases {
		data := &FieldData{
			Raw:    tc.Raw,
			Schema: tc.Schema,
		}

		actual := data.Get(tc.Key)
		if !reflect.DeepEqual(actual, tc.Value) {
			t.Fatalf(
				"bad: %s\n\nExpected: %#v\nGot: %#v",
				name, tc.Value, actual)
		}
	}
}
