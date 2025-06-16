// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestIdentityClaims_IsNode(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputIdentityClaims *IdentityClaims
		expectedOutput      bool
	}{
		{
			name:                "nil identity claims",
			inputIdentityClaims: nil,
			expectedOutput:      false,
		},
		{
			name:                "no identity claims",
			inputIdentityClaims: &IdentityClaims{},
			expectedOutput:      false,
		},
		{
			name: "workload identity claims",
			inputIdentityClaims: &IdentityClaims{
				WorkloadIdentityClaims: &WorkloadIdentityClaims{},
			},
			expectedOutput: false,
		},
		{
			name: "node identity claims",
			inputIdentityClaims: &IdentityClaims{
				NodeIdentityClaims: &NodeIdentityClaims{},
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsNode()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestIdentityClaims_IsWorkload(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputIdentityClaims *IdentityClaims
		expectedOutput      bool
	}{
		{
			name:                "nil identity claims",
			inputIdentityClaims: nil,
			expectedOutput:      false,
		},
		{
			name:                "no identity claims",
			inputIdentityClaims: &IdentityClaims{},
			expectedOutput:      false,
		},
		{
			name: "node identity claims",
			inputIdentityClaims: &IdentityClaims{
				NodeIdentityClaims: &NodeIdentityClaims{},
			},
			expectedOutput: false,
		},
		{
			name: "workload identity claims",
			inputIdentityClaims: &IdentityClaims{
				WorkloadIdentityClaims: &WorkloadIdentityClaims{},
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsWorkload()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestIdentityClaims_IsExpiring(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputIdentityClaims *IdentityClaims
		inputNow            time.Time
		inputTTL            time.Duration
		expectedResult      bool
	}{
		{
			name:                "nil identity",
			inputIdentityClaims: nil,
			inputNow:            time.Now(),
			inputTTL:            10 * time.Minute,
			expectedResult:      false,
		},
		{
			name:                "no expiry",
			inputIdentityClaims: &IdentityClaims{},
			inputNow:            time.Now(),
			inputTTL:            10 * time.Minute,
			expectedResult:      false,
		},
		{
			name: "not expiring not close",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now().Add(100 * time.Hour)),
				},
			},
			inputNow:       time.Now(),
			inputTTL:       100 * time.Hour,
			expectedResult: false,
		},
		{
			name: "not expiring close",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now().Add(100 * time.Hour)),
				},
			},
			inputNow:       time.Now().Add(30 * time.Hour),
			inputTTL:       100 * time.Hour,
			expectedResult: false,
		},
		{
			name: "expired close",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now().Add(100 * time.Hour)),
				},
			},
			inputNow:       time.Now().Add(67 * time.Hour),
			inputTTL:       100 * time.Hour,
			expectedResult: true,
		},
		{
			name: "expired not close",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now().Add(100 * time.Hour)),
				},
			},
			inputNow:       time.Now().Add(200 * time.Hour),
			inputTTL:       100 * time.Hour,
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsExpiring(tc.inputNow, tc.inputTTL)
			must.Eq(t, tc.expectedResult, actualOutput)
		})
	}
}

func TestIdentityClaimsNg_setExpiry(t *testing.T) {
	ci.Parallel(t)

	timeNow := time.Now().UTC()
	ttl := 10 * time.Minute

	claims := IdentityClaims{}
	claims.setExpiry(timeNow, ttl)

	// Round the time to the nearest minute for comparison, to accommodate
	// potential time differences in the test environment caused by function
	// call overhead. This can be seen when running a suite of tests in
	// parallel.
	must.Eq(t, timeNow.Add(ttl).Round(time.Minute),
		claims.Expiry.Time().UTC().Round(time.Minute))
}

func TestIdentityClaimsNg_setNodeSubject(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name            string
		inputNode       *Node
		inputRegion     string
		expectedSubject string
	}{
		{
			name: "global region",
			inputNode: &Node{
				ID:       "node-id-1",
				NodePool: "default",
			},
			inputRegion:     "global",
			expectedSubject: "node:global:default:node-id-1:default",
		},
		{
			name: "eu1 region",
			inputNode: &Node{
				ID:       "node-id-2",
				NodePool: "nlp",
			},
			inputRegion:     "eu1",
			expectedSubject: "node:eu1:nlp:node-id-2:default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			claims := IdentityClaims{}
			claims.setNodeSubject(tc.inputNode, tc.inputRegion)
			must.Eq(t, tc.expectedSubject, claims.Subject)
		})
	}
}
