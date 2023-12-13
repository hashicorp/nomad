// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package flatmap

import (
	"reflect"
	"testing"
)

type simpleTypes struct {
	b    bool
	i    int
	i8   int8
	i16  int16
	i32  int32
	i64  int64
	ui   uint
	ui8  uint8
	ui16 uint16
	ui32 uint32
	ui64 uint64
	f32  float32
	f64  float64
	c64  complex64
	c128 complex128
	s    string
}

type linkedList struct {
	value string
	next  *linkedList
}

type containers struct {
	myslice []int
	mymap   map[string]linkedList
}

type interfaceHolder struct {
	value interface{}
}

func TestFlatMap(t *testing.T) {
	cases := []struct {
		Input         interface{}
		Expected      map[string]string
		Filter        []string
		PrimitiveOnly bool
	}{
		{
			Input:    nil,
			Expected: nil,
		},
		{
			Input: &simpleTypes{
				b:    true,
				i:    -10,
				i8:   88,
				i16:  1616,
				i32:  3232,
				i64:  6464,
				ui:   10,
				ui8:  88,
				ui16: 1616,
				ui32: 3232,
				ui64: 6464,
				f32:  3232,
				f64:  6464,
				c64:  64,
				c128: 128,
				s:    "foobar",
			},
			Expected: map[string]string{
				"b":    "true",
				"i":    "-10",
				"i8":   "88",
				"i16":  "1616",
				"i32":  "3232",
				"i64":  "6464",
				"ui":   "10",
				"ui8":  "88",
				"ui16": "1616",
				"ui32": "3232",
				"ui64": "6464",
				"f32":  "3232",
				"f64":  "6464",
				"c64":  "(64+0i)",
				"c128": "(128+0i)",
				"s":    "foobar",
			},
		},
		{
			Input: &simpleTypes{
				b:    true,
				i:    -10,
				i8:   88,
				i16:  1616,
				i32:  3232,
				i64:  6464,
				ui:   10,
				ui8:  88,
				ui16: 1616,
				ui32: 3232,
				ui64: 6464,
				f32:  3232,
				f64:  6464,
				c64:  64,
				c128: 128,
				s:    "foobar",
			},
			Filter: []string{"i", "i8", "i16"},
			Expected: map[string]string{
				"b":    "true",
				"i32":  "3232",
				"i64":  "6464",
				"ui":   "10",
				"ui8":  "88",
				"ui16": "1616",
				"ui32": "3232",
				"ui64": "6464",
				"f32":  "3232",
				"f64":  "6464",
				"c64":  "(64+0i)",
				"c128": "(128+0i)",
				"s":    "foobar",
			},
		},
		{
			Input: &linkedList{
				value: "foo",
				next: &linkedList{
					value: "bar",
					next:  nil,
				},
			},
			Expected: map[string]string{
				"value":      "foo",
				"next.value": "bar",
				"next.next":  "nil",
			},
		},
		{
			Input: &linkedList{
				value: "foo",
				next: &linkedList{
					value: "bar",
					next:  nil,
				},
			},
			PrimitiveOnly: true,
			Expected: map[string]string{
				"value": "foo",
			},
		},
		{
			Input: linkedList{
				value: "foo",
				next: &linkedList{
					value: "bar",
					next:  nil,
				},
			},
			PrimitiveOnly: true,
			Expected: map[string]string{
				"value": "foo",
			},
		},
		{
			Input: &containers{
				myslice: []int{1, 2},
				mymap: map[string]linkedList{
					"foo": {
						value: "l1",
					},
					"bar": {
						value: "l2",
					},
				},
			},
			Expected: map[string]string{
				"myslice[0]":       "1",
				"myslice[1]":       "2",
				"mymap[foo].value": "l1",
				"mymap[foo].next":  "nil",
				"mymap[bar].value": "l2",
				"mymap[bar].next":  "nil",
			},
		},
		{
			Input: &containers{
				myslice: []int{1, 2},
				mymap: map[string]linkedList{
					"foo": {
						value: "l1",
					},
					"bar": {
						value: "l2",
					},
				},
			},
			PrimitiveOnly: true,
			Expected:      map[string]string{},
		},
		{
			Input: &interfaceHolder{
				value: &linkedList{
					value: "foo",
					next:  nil,
				},
			},
			Expected: map[string]string{
				"value.value": "foo",
				"value.next":  "nil",
			},
		},
		{
			Input: &interfaceHolder{
				value: &linkedList{
					value: "foo",
					next:  nil,
				},
			},
			PrimitiveOnly: true,
			Expected:      map[string]string{},
		},
	}

	for i, c := range cases {
		act := Flatten(c.Input, c.Filter, c.PrimitiveOnly)
		if !reflect.DeepEqual(act, c.Expected) {
			t.Fatalf("case %d: got %#v; want %#v", i+1, act, c.Expected)
		}
	}
}
