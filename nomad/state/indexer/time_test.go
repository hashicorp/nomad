// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexer

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func Test_IndexFromTimeQuery(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputArg            interface{}
		expectedOutputBytes []byte
		expectedOutputError error
		name                string
	}{
		{
			inputArg: &TimeQuery{
				Value: time.Date(1987, time.April, 13, 8, 3, 0, 0, time.UTC),
			},
			expectedOutputBytes: []byte{0x0, 0x0, 0x0, 0x0, 0x20, 0x80, 0x9b, 0xb4},
			expectedOutputError: nil,
			name:                "generic test 1",
		},
		{
			inputArg: &TimeQuery{
				Value: time.Date(2022, time.April, 27, 14, 12, 0, 0, time.UTC),
			},
			expectedOutputBytes: []byte{0x0, 0x0, 0x0, 0x0, 0x62, 0x69, 0x4f, 0x30},
			expectedOutputError: nil,
			name:                "generic test 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, actualError := IndexFromTimeQuery(tc.inputArg)
			require.Equal(t, tc.expectedOutputError, actualError)
			require.Equal(t, tc.expectedOutputBytes, actualOutput)
		})
	}
}
