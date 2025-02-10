// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package validators

import (
	"fmt"
	"os/user"
	"strconv"
	"testing"

	"github.com/hashicorp/go-hclog"
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
			err := validateIDRange("uid", tc.idRange)
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

	user, err := user.Current()
	must.NoError(t, err)

	userID, err := strconv.ParseUint(user.Uid, 10, 32)
	groupID, err := strconv.ParseUint(user.Gid, 10, 32)
	must.NoError(t, err)

	userNotIncluded := fmt.Sprintf("%d-%d", userID+1, userID+11)
	userIncluded := fmt.Sprintf("%d-%d", userID, userID+11)
	userNotIncludedSingle := fmt.Sprintf("%d", userID+1)

	groupNotIncluded := fmt.Sprintf("%d-%d", groupID+1, groupID+11)
	groupIncluded := fmt.Sprintf("%d-%d", groupID, groupID+11)
	groupNotIncludedSingle := fmt.Sprintf("%d", groupID+1)

	emptyRanges := ""

	userDeniedRangesList := fmt.Sprintf("%s,%s", userNotIncluded, userNotIncludedSingle)
	groupDeniedRangesList := fmt.Sprintf("%s,%s", groupNotIncluded, groupNotIncludedSingle)

	testCases := []struct {
		name        string
		uidRanges   string
		gidRanges   string
		expectedErr string
	}{
		{name: "user_not_in_denied_ranges", uidRanges: userDeniedRangesList, gidRanges: emptyRanges},
		{name: "user_and group_not_in_denied_ranges", uidRanges: userDeniedRangesList, gidRanges: groupDeniedRangesList},
		{name: "uid_in_one_of_ranges_is_invalid", uidRanges: userIncluded, gidRanges: groupDeniedRangesList, expectedErr: fmt.Sprintf("running as uid %s is disallowed", user.Uid)},
		{name: "gid-in-one-of-ranges-is-invalid", uidRanges: userDeniedRangesList, gidRanges: groupIncluded, expectedErr: fmt.Sprintf("running as gid %s is disallowed", user.Gid)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewValidator(hclog.NewNullLogger(), tc.uidRanges, tc.gidRanges)
			must.NoError(t, err)

			err = v.HasValidIDs(user.Username)

			if tc.expectedErr == "" {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func Test_ValidateBounds(t *testing.T) {
	testCases := []struct {
		name        string
		bounds      string
		expectedErr error
	}{
		{name: "invalid_bound", bounds: "banana", expectedErr: ErrInvalidBound},
		{name: "invalid_lower_bound", bounds: "banana-10", expectedErr: ErrInvalidBound},
		{name: "invalid_upper_bound", bounds: "10-banana", expectedErr: ErrInvalidBound},
		{name: "lower_bigger_than_upper", bounds: "10-1", expectedErr: ErrInvalidRange},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBounds(tc.bounds)
			must.ErrorIs(t, err, tc.expectedErr)
		})
	}
}
