// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func Test_Backoff(t *testing.T) {
	const MaxUint = ^uint64(0)
	const MaxInt = int64(MaxUint >> 1)

	cases := []struct {
		name           string
		backoffBase    time.Duration
		backoffLimit   time.Duration
		attempt        uint64
		expectedResult time.Duration
	}{
		{
			name:           "backoff limit clamps for high base",
			backoffBase:    time.Hour,
			backoffLimit:   time.Minute,
			attempt:        1,
			expectedResult: time.Minute,
		},
		{
			name:           "backoff limit clamps for boundary attempt",
			backoffBase:    time.Hour,
			backoffLimit:   time.Minute,
			attempt:        63,
			expectedResult: time.Minute,
		},
		{
			name:           "small retry value",
			backoffBase:    time.Minute,
			backoffLimit:   time.Hour,
			attempt:        0,
			expectedResult: time.Minute,
		},
		{
			name:           "first retry value",
			backoffBase:    time.Minute,
			backoffLimit:   time.Hour,
			attempt:        1,
			expectedResult: 2 * time.Minute,
		},
		{
			name:           "fifth retry value",
			backoffBase:    time.Minute,
			backoffLimit:   time.Hour,
			attempt:        5,
			expectedResult: 32 * time.Minute,
		},
		{
			name:           "sixth retry value",
			backoffBase:    time.Minute,
			backoffLimit:   time.Hour,
			attempt:        6,
			expectedResult: time.Hour,
		},
	}

	for _, tc := range cases {
		result := Backoff(tc.backoffBase, tc.backoffLimit, tc.attempt)
		must.Eq(t, tc.expectedResult, result)
	}
}
