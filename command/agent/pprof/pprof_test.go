// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pprof

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestProfile(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc            string
		profile         string
		debug           int
		gc              int
		expectedHeaders map[string]string
		expectedErr     error
	}{
		{
			desc:    "profile that exists",
			profile: "goroutine",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Content-Type":           "application/octet-stream",
				"Content-Disposition":    `attachment; filename="goroutine"`,
			},
		},
		{
			desc:            "profile that does not exist",
			profile:         "nonexistent",
			expectedErr:     NewErrProfileNotFound("nonexistent"),
			expectedHeaders: nil,
		},
		{
			desc:    "profile with debug enabled",
			profile: "allocs",
			debug:   1,
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Content-Type":           "text/plain; charset=utf-8",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			resp, headers, err := Profile(tc.profile, tc.debug, tc.gc)
			require.Equal(t, tc.expectedHeaders, headers)

			if tc.expectedErr != nil {
				require.Nil(t, resp)
				require.Equal(t, err, tc.expectedErr)
			} else {
				require.NotNil(t, resp)
			}
		})
	}
}

func TestCPUProfile(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc            string
		expectedHeaders map[string]string
	}{
		{
			desc: "successful cpu profile",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Content-Type":           "application/octet-stream",
				"Content-Disposition":    `attachment; filename="profile"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			resp, headers, err := CPUProfile(context.Background(), 0)
			require.NoError(t, err)
			require.Equal(t, tc.expectedHeaders, headers)

			require.NotNil(t, resp)
		})
	}
}

func TestTrace(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc            string
		expectedHeaders map[string]string
	}{
		{
			desc: "successful trace profile",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Content-Type":           "application/octet-stream",
				"Content-Disposition":    `attachment; filename="trace"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			resp, headers, err := Trace(context.Background(), 0)
			require.NoError(t, err)
			require.Equal(t, tc.expectedHeaders, headers)

			require.NotNil(t, resp)
		})
	}
}

func TestCmdline(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc            string
		expectedHeaders map[string]string
	}{
		{
			desc: "successful cmdline request",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Content-Type":           "text/plain; charset=utf-8",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			resp, headers, err := Cmdline()
			require.NoError(t, err)
			require.Equal(t, tc.expectedHeaders, headers)

			require.NotNil(t, resp)
		})
	}
}
