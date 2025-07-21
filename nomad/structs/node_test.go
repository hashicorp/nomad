// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestDriverInfoEquals(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	var driverInfoTest = []struct {
		input    []*DriverInfo
		expected bool
		errorMsg string
	}{
		{
			[]*DriverInfo{
				{
					Healthy: true,
				},
				{
					Healthy: false,
				},
			},
			false,
			"Different healthy values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					HealthDescription: "not running",
				},
				{
					HealthDescription: "running",
				},
			},
			false,
			"Different health description values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					Detected:          false,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
				{
					Detected:          true,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
			},
			true,
			"Same health check should be equal",
		},
	}
	for _, testCase := range driverInfoTest {
		first := testCase.input[0]
		second := testCase.input[1]
		require.Equal(testCase.expected, first.HealthCheckEquals(second), testCase.errorMsg)
	}
}

func TestNodeMeta_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		input    map[string]*string // only specify Meta field
		contains string
	}{
		{
			name: "Ok",
			input: map[string]*string{
				"foo":             nil,
				"bar":             nil,
				"eggs":            nil,
				"dots.are_ok-too": nil,
			},
		},
		{
			name:     "Nil",
			input:    nil,
			contains: "missing required",
		},
		{
			name:     "Empty",
			input:    map[string]*string{},
			contains: "missing required",
		},
		{
			name:     "EmptyKey",
			input:    map[string]*string{"": nil},
			contains: "not be empty",
		},
		{
			name: "Whitespace",
			input: map[string]*string{
				"ok":   nil,
				" bad": nil,
			},
			contains: `" bad" is invalid`,
		},
		{
			name: "BadChars",
			input: map[string]*string{
				"ok":    nil,
				"*bad%": nil,
			},
			contains: `"*bad%" is invalid`,
		},
		{
			name: "StartingDot",
			input: map[string]*string{
				"ok":   nil,
				".bad": nil,
			},
			contains: `".bad" is invalid`,
		},
		{
			name: "EndingDot",
			input: map[string]*string{
				"ok":   nil,
				"bad.": nil,
			},
			contains: `"bad." is invalid`,
		},
		{
			name: "DottedPartsMustBeValid",
			input: map[string]*string{
				"ok":        nil,
				"bad.-part": nil,
			},
			contains: `"bad.-part" is invalid`,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			in := &NodeMetaApplyRequest{
				Meta: tc.input,
			}

			err := in.Validate()

			switch tc.contains {
			case "":
				must.NoError(t, err)
			default:
				must.ErrorContains(t, err, tc.contains)

				// Log error to make it easy to double check output.
				t.Logf("Validate(%s) -> %s", tc.name, err)
			}
		})
	}
}

func TestCSITopology_Contains(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		this     *CSITopology
		other    *CSITopology
		expected bool
	}{
		{
			name: "AWS EBS pre 1.27 behavior",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
				},
			},
			expected: true,
		},
		{
			name: "AWS EBS post 1.27 behavior",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone":   "us-east-1a",
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
					"kubernetes.io/os":              "linux",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			expected: true,
		},
		{
			name: "other contains invalid segment value for matched key",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone":   "us-east-1a",
					"topology.ebs.csi.aws.com/zone": "us-east-1a",
					"kubernetes.io/os":              "linux",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
					"kubernetes.io/os":            "windows",
				},
			},
			expected: false,
		},
		{
			name: "other contains invalid segment key",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			other: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
					"kubernetes.io/os":            "linux",
				},
			},
			expected: false,
		},
		{
			name: "other is nil",
			this: &CSITopology{
				Segments: map[string]string{
					"topology.kubernetes.io/zone": "us-east-1a",
				},
			},
			other:    nil,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, tc.this.Contains(tc.other))
		})
	}

}

func TestGenerateNodeIdentityClaims(t *testing.T) {
	ci.Parallel(t)

	node := &Node{
		ID:         "node-id-1",
		NodePool:   "custom-pool",
		NodeClass:  "custom-class",
		Datacenter: "euw2",
	}

	claims := GenerateNodeIdentityClaims(node, "euw", 10*time.Minute)

	must.Eq(t, "node-id-1", claims.NodeIdentityClaims.NodeID)
	must.Eq(t, "custom-pool", claims.NodeIdentityClaims.NodePool)
	must.Eq(t, "custom-class", claims.NodeIdentityClaims.NodeClass)
	must.Eq(t, "euw2", claims.NodeIdentityClaims.NodeDatacenter)
	must.StrEqFold(t, "node:euw:custom-pool:node-id-1:default", claims.Subject)
	must.Eq(t, []string{IdentityDefaultAud}, claims.Audience)
	must.NotNil(t, claims.ID)
	must.NotNil(t, claims.IssuedAt)
	must.NotNil(t, claims.NotBefore)
	must.NotNil(t, claims.Expiry)
}

func TestNodeRegisterRequest_ShouldGenerateNodeIdentity(t *testing.T) {
	ci.Parallel(t)

	// Generate a stable mock node for testing.
	mockNode := MockNode()

	testCases := []struct {
		name                     string
		inputNodeRegisterRequest *NodeRegisterRequest
		inputAuthErr             error
		inputTime                time.Time
		inputTTL                 time.Duration
		expectedOutput           bool
	}{
		{
			name:                     "expired node identity",
			inputNodeRegisterRequest: &NodeRegisterRequest{},
			inputAuthErr:             jwt.ErrExpired,
			inputTime:                time.Now(),
			inputTTL:                 10 * time.Minute,
			expectedOutput:           true,
		},
		{
			name: "first time node registration",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ACLToken: AnonymousACLToken,
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now(),
			inputTTL:       10 * time.Minute,
			expectedOutput: true,
		},
		{
			name: "registration using node secret ID",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ClientID: "client-id-1",
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now(),
			inputTTL:       10 * time.Minute,
			expectedOutput: true,
		},
		{
			name: "modified node node pool configuration",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				Node: mockNode,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{
								NodeID:         mockNode.ID,
								NodePool:       "new-pool",
								NodeClass:      mockNode.NodeClass,
								NodeDatacenter: mockNode.Datacenter,
							},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(23 * time.Hour)),
							},
						},
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "modified node class configuration",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				Node: mockNode,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{
								NodeID:         mockNode.ID,
								NodePool:       mockNode.NodePool,
								NodeClass:      "new-class",
								NodeDatacenter: mockNode.Datacenter,
							},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(23 * time.Hour)),
							},
						},
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "modified node datacenter configuration",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				Node: mockNode,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{
								NodeID:         mockNode.ID,
								NodePool:       mockNode.NodePool,
								NodeClass:      mockNode.NodeClass,
								NodeDatacenter: "new-datacenter",
							},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(23 * time.Hour)),
							},
						},
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "expiring node identity",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				Node: mockNode,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{
								NodeID:         mockNode.ID,
								NodePool:       mockNode.NodePool,
								NodeClass:      mockNode.NodeClass,
								NodeDatacenter: mockNode.Datacenter,
							},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
							},
						},
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "no generation",
			inputNodeRegisterRequest: &NodeRegisterRequest{
				Node: mockNode,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{
								NodeID:         mockNode.ID,
								NodePool:       mockNode.NodePool,
								NodeClass:      mockNode.NodeClass,
								NodeDatacenter: mockNode.Datacenter,
							},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(24 * time.Hour)),
							},
						},
					},
				},
			},
			inputAuthErr:   nil,
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputNodeRegisterRequest.ShouldGenerateNodeIdentity(
				tc.inputAuthErr,
				tc.inputTime,
				tc.inputTTL,
			)
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestNodeUpdateStatusRequest_ShouldGenerateNodeIdentity(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                     string
		inputNodeRegisterRequest *NodeUpdateStatusRequest
		inputTime                time.Time
		inputTTL                 time.Duration
		expectedOutput           bool
	}{
		{
			name: "authenticated by node secret ID",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ClientID: "client-id-1",
					},
				},
			},
			inputTime:      time.Now(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "expiring node identity",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(1 * time.Hour)),
							},
						},
					},
				},
			},
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "not expiring node identity",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(24 * time.Hour)),
							},
						},
					},
				},
			},
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: false,
		},
		{
			name: "not expiring forced renewal node identity",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				ForceIdentityRenewal: true,
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(24 * time.Hour)),
							},
						},
					},
				},
			},
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: true,
		},
		{
			name: "server authenticated request",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ACLToken: LeaderACLToken,
					},
				},
			},
			inputTime:      time.Now().UTC(),
			inputTTL:       24 * time.Hour,
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputNodeRegisterRequest.ShouldGenerateNodeIdentity(
				tc.inputTime,
				tc.inputTTL,
			)
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}
func TestNodeUpdateStatusRequest_IdentitySigningErrorIsTerminal(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                     string
		inputNodeRegisterRequest *NodeUpdateStatusRequest
		inputTime                time.Time
		expectedOutput           bool
	}{
		{
			name: "not close to expiring",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC().Add(24 * time.Hour).UTC()),
							},
						},
					},
				},
			},
			inputTime:      time.Now().UTC(),
			expectedOutput: false,
		},
		{
			name: "very close to expiring",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						Claims: &IdentityClaims{
							NodeIdentityClaims: &NodeIdentityClaims{},
							Claims: jwt.Claims{
								Expiry: jwt.NewNumericDate(time.Now().UTC()),
							},
						},
					},
				},
			},
			inputTime:      time.Now().Add(1 * time.Minute).UTC(),
			expectedOutput: true,
		},
		{
			name: "server authenticated request",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ACLToken: LeaderACLToken,
					},
				},
			},
			inputTime:      time.Now().UTC(),
			expectedOutput: false,
		},
		{
			name: "client secret ID authenticated request",
			inputNodeRegisterRequest: &NodeUpdateStatusRequest{
				WriteRequest: WriteRequest{
					identity: &AuthenticatedIdentity{
						ClientID: "client-id",
					},
				},
			},
			inputTime:      time.Now().UTC(),
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputNodeRegisterRequest.IdentitySigningErrorIsTerminal(tc.inputTime)
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func Test_DefaultNodeIntroductionConfig(t *testing.T) {
	ci.Parallel(t)

	expected := &NodeIntroductionConfig{
		Enforcement:        "warn",
		DefaultIdentityTTL: 5 * time.Minute,
		MaxIdentityTTL:     30 * time.Minute,
	}
	must.Eq(t, expected, DefaultNodeIntroductionConfig())
}

func TestNodeIntroductionConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	nodeIntro := &NodeIntroductionConfig{
		Enforcement:        "warn",
		DefaultIdentityTTL: 5 * time.Minute,
		MaxIdentityTTL:     30 * time.Minute,
	}

	copiedNodeIntro := nodeIntro.Copy()

	// Ensure the copied object contains the same values, but the underlying
	// pointer address is different.
	must.Eq(t, nodeIntro, copiedNodeIntro)
	must.NotEq(t, fmt.Sprintf("%p", nodeIntro), fmt.Sprintf("%p", copiedNodeIntro))
}

func TestNodeIntroductionConfig_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                        string
		inputNodeIntroductionConfig *NodeIntroductionConfig
		expectedErrorContains       string
	}{
		{
			name:                        "nil config",
			inputNodeIntroductionConfig: nil,
			expectedErrorContains:       "cannot be empty",
		},
		{
			name: "incorrect enforcement",
			inputNodeIntroductionConfig: &NodeIntroductionConfig{
				Enforcement:        "invalid",
				DefaultIdentityTTL: 5 * time.Minute,
				MaxIdentityTTL:     30 * time.Minute,
			},
			expectedErrorContains: "invalid enforcement",
		},
		{
			name: "incorrect default identity TTL",
			inputNodeIntroductionConfig: &NodeIntroductionConfig{
				Enforcement:        NodeIntroductionEnforcementStrict,
				DefaultIdentityTTL: 0,
				MaxIdentityTTL:     30 * time.Minute,
			},
			expectedErrorContains: "default_identity_ttl must be greater than 0",
		},
		{
			name: "incorrect max identity TTL",
			inputNodeIntroductionConfig: &NodeIntroductionConfig{
				Enforcement:        NodeIntroductionEnforcementStrict,
				DefaultIdentityTTL: 5 * time.Minute,
				MaxIdentityTTL:     0,
			},
			expectedErrorContains: "max_identity_ttl must be greater than 0",
		},
		{
			name: "incorrect max identity TTL greater than default identity TTL",
			inputNodeIntroductionConfig: &NodeIntroductionConfig{
				Enforcement:        NodeIntroductionEnforcementStrict,
				DefaultIdentityTTL: 5 * time.Minute,
				MaxIdentityTTL:     0,
			},
			expectedErrorContains: "max_identity_ttl must be greater than or equal to default_identity_ttl",
		},
		{
			name: "valid",
			inputNodeIntroductionConfig: &NodeIntroductionConfig{
				Enforcement:        NodeIntroductionEnforcementStrict,
				DefaultIdentityTTL: 15 * time.Minute,
				MaxIdentityTTL:     45 * time.Minute,
			},
			expectedErrorContains: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.inputNodeIntroductionConfig.Validate()
			if tc.expectedErrorContains == "" {
				must.NoError(t, actualError)
			} else {
				must.ErrorContains(t, actualError, tc.expectedErrorContains)
			}
		})
	}
}
