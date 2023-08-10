// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package idset

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Parse(t *testing.T) {
	cases := []struct {
		input string
		exp   []uint16
	}{
		{
			input: "0",
			exp:   []uint16{0},
		},
		{
			input: "1,3,5,9",
			exp:   []uint16{1, 3, 5, 9},
		},
		{
			input: "1-2",
			exp:   []uint16{1, 2},
		},
		{
			input: "3-6",
			exp:   []uint16{3, 4, 5, 6},
		},
		{
			input: "1,3-5,9,11-14",
			exp:   []uint16{1, 3, 4, 5, 9, 11, 12, 13, 14},
		},
		{
			input: " 4-2 , 9-9 , 11-7\n",
			exp:   []uint16{2, 3, 4, 7, 8, 9, 10, 11},
		},
	}

	for _, tc := range cases {
		t.Run("("+tc.input+")", func(t *testing.T) {
			result := Parse[uint16](tc.input).Slice()
			must.SliceContainsAll(t, tc.exp, result, must.Sprint("got", result))
		})
	}
}

func Test_String(t *testing.T) {
	cases := []struct {
		input string
		exp   string
	}{
		{
			input: "0",
			exp:   "0",
		},
		{
			input: "1-3",
			exp:   "1-3",
		},
		{
			input: "1, 2, 3",
			exp:   "1-3",
		},
		{
			input: "7, 1-3, 12-9",
			exp:   "1-3,7,9-12",
		},
	}

	for _, tc := range cases {
		t.Run("("+tc.input+")", func(t *testing.T) {
			result := Parse[uint16](tc.input)
			str := result.String()
			must.Eq(t, tc.exp, str, must.Sprint("slice", result.Slice()))
		})
	}
}
