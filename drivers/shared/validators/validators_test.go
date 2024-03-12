// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"os/user"
	"testing"

	"github.com/shoenig/test/must"
)

var validRangeStr = "1-100"
var validRangeSingleStr = "1"
var flippedBoundsMessage = "lower bound cannot be greater than upper bound"
var invalidRangeFlipped = "100-1"

var invalidBound = "range bound not valid"
var invalidRangeSubstring = "1-100,foo"
var invalidRangeEmpty = "1-100,,200-300"

func Test_IDRangeValid(t *testing.T) {
	testCases := []struct {
		name        string
		idRange     string
		expectedErr string
	}{
		{name: "standard-range-is-valid", idRange: validRangeStr},
		{name: "same-number-for-both-bounds-is-valid", idRange: validRangeSingleStr},
		{name: "lower-higher-than-upper-is-invalid", idRange: invalidRangeFlipped, expectedErr: flippedBoundsMessage},
		{name: "missing-lower-is-invalid", idRange: invalidRangeSubstring, expectedErr: invalidBound},
		{name: "missing-higher-is-invalid", idRange: invalidRangeEmpty, expectedErr: invalidBound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseIdRange("uid", tc.idRange)
			if tc.expectedErr == "" {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func Test_HasValidIds(t *testing.T) {
	var validRange = IDRange{
		Lower: 1,
		Upper: 100,
	}

	var validRangeSingle = IDRange{
		Lower: 1,
		Upper: 1,
	}

	emptyRanges := []IDRange{}
	validRangesList := []IDRange{validRange, validRangeSingle}

	testCases := []struct {
		name        string
		uidRanges   []IDRange
		gidRanges   []IDRange
		uid         string
		gid         string
		expectedErr string
	}{
		{name: "no-ranges-are-valid", uidRanges: validRangesList, gidRanges: emptyRanges},
		{name: "uid-and-gid-outside-of-ranges-valid", uidRanges: validRangesList, gidRanges: validRangesList},
		{name: "uid-in-one-of-ranges-is-invalid", uidRanges: validRangesList, gidRanges: validRangesList, uid: "50", expectedErr: "running as uid 50 is disallowed"},
		{name: "gid-in-one-of-ranges-is-invalid", uidRanges: validRangesList, gidRanges: validRangesList, gid: "50", expectedErr: "running as gid 50 is disallowed"},
		{name: "string-uid-throws-error", uid: "banana", expectedErr: "unable to convert userid banana to integer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &user.User{
				Uid: "200",
				Gid: "200",
			}

			if tc.uid != "" {
				user.Uid = tc.uid
			}

			if tc.gid != "" {
				user.Gid = tc.gid
			}

			err := HasValidIds(user, tc.uidRanges, tc.gidRanges)
			if tc.expectedErr == "" {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}
