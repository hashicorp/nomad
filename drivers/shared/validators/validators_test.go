// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package validators

import (
	"os/user"
	"testing"

	"github.com/shoenig/test/must"
)

var validRange = "1-100"
var validRangeSingle = "1"
var flippedBoundsMessage = "lower bound cannot be greater than upper bound"
var invalidRangeFlipped = "100-1"

var invalidBound = "range bound not valid"
var invalidRangeSubstring = "1-100,foo"
var invalidRangeEmpty = "1-100,,200-300"

func Test_IDRangeValid(t *testing.T) {
	testCases := []struct {
		name         string
		idRange      string
		expectedPass bool
		expectedErr  string
	}{
		{name: "standard-range-is-valid", idRange: validRange, expectedPass: true},
		{name: "same-number-for-both-bounds-is-valid", idRange: validRangeSingle, expectedPass: true},
		{name: "lower-higher-than-upper-is-invalid", idRange: invalidRangeFlipped, expectedPass: false, expectedErr: flippedBoundsMessage},
		{name: "missing-lower-is-invalid", idRange: invalidRangeSubstring, expectedPass: false, expectedErr: invalidBound},
		{name: "missing-higher-is-invalid", idRange: invalidRangeEmpty, expectedPass: false, expectedErr: invalidBound},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := IDRangeValid("uid", tc.idRange)
			if tc.expectedPass {
				must.NoError(t, err)
			} else {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					must.StrContains(t, err.Error(), tc.expectedErr)
				}
			}
		})
	}
}

func Test_UserInRange(t *testing.T) {
	emptyRange := ""
	invalidRange := "foo"

	testCases := []struct {
		name           string
		uidRanges      string
		gidRanges      string
		uid            string
		gid            string
		expectedPass   bool
		expectedErr    string
		userLookupFunc userLookupFn
	}{
		{name: "no-ranges-are-valid", uidRanges: emptyRange, gidRanges: emptyRange, expectedPass: true},
		{name: "uid-and-gid-outside-of-ranges-valid", uidRanges: validRange, gidRanges: validRange, expectedPass: true},
		{name: "uid-in-one-of-ranges-is-invalid", uidRanges: validRange, gidRanges: validRange, uid: "50", expectedPass: false, expectedErr: "running as uid 50 is disallowed"},
		{name: "gid-in-one-of-ranges-is-invalid", uidRanges: validRange, gidRanges: validRange, gid: "50", expectedPass: false, expectedErr: "running as gid 50 is disallowed"},
		{name: "invalid-uid-range-throws-error", uidRanges: invalidRange, gidRanges: validRange, expectedPass: false, expectedErr: "invalid denied_host_uids value"},
		{name: "invalid-gid-range-throws-error", uidRanges: validRange, gidRanges: invalidRange, expectedPass: false, expectedErr: "invalid denied_host_gids value"},
		{name: "string-uid-throws-error", uid: "banana", expectedPass: false, expectedErr: "unable to convert userid banana to integer"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defaultUserToReturn := &user.User{
				Uid: "200",
				Gid: "200",
			}

			if tc.uid != "" {
				defaultUserToReturn.Uid = tc.uid
			}

			if tc.gid != "" {
				defaultUserToReturn.Gid = tc.gid
			}

			getUserFn := func(username string) (*user.User, error) {
				return defaultUserToReturn, nil
			}

			err := UserInRange(getUserFn, "username", tc.uidRanges, tc.gidRanges)
			if tc.expectedPass {
				must.NoError(t, err)
			} else {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					must.StrContains(t, err.Error(), tc.expectedErr)
				}
			}
		})
	}
}
