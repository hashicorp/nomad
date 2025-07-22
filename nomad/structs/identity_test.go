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
		{
			name: "node introduction identity claims",
			inputIdentityClaims: &IdentityClaims{
				NodeIntroductionIdentityClaims: &NodeIntroductionIdentityClaims{},
			},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsNode()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestIdentityClaims_IsNodeIntroduction(t *testing.T) {
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
			expectedOutput: false,
		},
		{
			name: "node introduction identity claims",
			inputIdentityClaims: &IdentityClaims{
				NodeIntroductionIdentityClaims: &NodeIntroductionIdentityClaims{},
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsNodeIntroduction()
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
		{
			name: "node introduction identity claims",
			inputIdentityClaims: &IdentityClaims{
				NodeIntroductionIdentityClaims: &NodeIntroductionIdentityClaims{},
			},
			expectedOutput: false,
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

func TestIdentityClaims_IsExpiringWithTTL(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputIdentityClaims *IdentityClaims
		inputThreshold      time.Time
		expectedResult      bool
	}{
		{
			name:                "nil identity",
			inputIdentityClaims: nil,
			inputThreshold:      time.Now(),
			expectedResult:      false,
		},
		{
			name:                "no expiry",
			inputIdentityClaims: &IdentityClaims{},
			inputThreshold:      time.Now(),
			expectedResult:      false,
		},
		{
			name: "not close to expiring",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				},
			},
			inputThreshold: time.Now(),
			expectedResult: false,
		},
		{
			name: "close to expiring",
			inputIdentityClaims: &IdentityClaims{
				Claims: jwt.Claims{
					Expiry: jwt.NewNumericDate(time.Now()),
				},
			},
			inputThreshold: time.Now().Add(1 * time.Minute),
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputIdentityClaims.IsExpiringInThreshold(tc.inputThreshold)
			must.Eq(t, tc.expectedResult, actualOutput)
		})
	}
}

func TestIdentityClaims_setExpiry(t *testing.T) {
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

func TestIdentityClaims_setNodeSubject(t *testing.T) {
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
			claims := IdentityClaims{}
			claims.setNodeSubject(tc.inputNode, tc.inputRegion)
			must.Eq(t, tc.expectedSubject, claims.Subject)
		})
	}
}

func TestIdentityClaims_setNodeIntroductionSubject(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name            string
		inputName       string
		inputPool       string
		inputRegion     string
		expectedSubject string
	}{
		{
			name:            "eu1 region with node name",
			inputName:       "node-id-1",
			inputPool:       "nlp",
			inputRegion:     "eu1",
			expectedSubject: "node-introduction:eu1:nlp:node-id-1:default",
		},
		{
			name:            "eu1 region without node name",
			inputName:       "",
			inputPool:       "nlp",
			inputRegion:     "eu1",
			expectedSubject: "node-introduction:eu1:nlp:default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims := IdentityClaims{}
			claims.setNodeIntroductionSubject(tc.inputName, tc.inputPool, tc.inputRegion)
			must.Eq(t, tc.expectedSubject, claims.Subject)
		})
	}
}
