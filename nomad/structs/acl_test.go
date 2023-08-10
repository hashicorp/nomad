// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestACLToken_Canonicalize(t *testing.T) {
	testCases := []struct {
		name   string
		testFn func()
	}{
		{
			name: "token with accessor",
			testFn: func() {
				mockToken := &ACLToken{
					AccessorID:  uuid.Generate(),
					SecretID:    uuid.Generate(),
					Name:        "my cool token " + uuid.Generate(),
					Type:        "client",
					Policies:    []string{"foo", "bar"},
					Roles:       []*ACLTokenRoleLink{},
					Global:      false,
					CreateTime:  time.Now().UTC(),
					CreateIndex: 10,
					ModifyIndex: 20,
				}
				mockToken.SetHash()
				copiedMockToken := mockToken.Copy()

				mockToken.Canonicalize()
				require.Equal(t, copiedMockToken, mockToken)
			},
		},
		{
			name: "token without accessor",
			testFn: func() {
				mockToken := &ACLToken{
					Name:     "my cool token " + uuid.Generate(),
					Type:     "client",
					Policies: []string{"foo", "bar"},
					Global:   false,
				}

				mockToken.Canonicalize()
				require.NotEmpty(t, mockToken.AccessorID)
				require.NotEmpty(t, mockToken.SecretID)
				require.NotEmpty(t, mockToken.CreateTime)
			},
		},
		{
			name: "token with ttl without accessor",
			testFn: func() {
				mockToken := &ACLToken{
					Name:          "my cool token " + uuid.Generate(),
					Type:          "client",
					Policies:      []string{"foo", "bar"},
					Global:        false,
					ExpirationTTL: 10 * time.Hour,
				}

				mockToken.Canonicalize()
				require.NotEmpty(t, mockToken.AccessorID)
				require.NotEmpty(t, mockToken.SecretID)
				require.NotEmpty(t, mockToken.CreateTime)
				require.NotEmpty(t, mockToken.ExpirationTime)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestACLTokenValidate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		inputACLToken         *ACLToken
		inputExistingACLToken *ACLToken
		expectedErrorContains string
	}{
		{
			name:                  "missing type",
			inputACLToken:         &ACLToken{},
			inputExistingACLToken: nil,
			expectedErrorContains: "client or management",
		},
		{
			name: "missing policies or roles",
			inputACLToken: &ACLToken{
				Type: ACLClientToken,
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "missing policies or roles",
		},
		{
			name: "invalid policies",
			inputACLToken: &ACLToken{
				Type:     ACLManagementToken,
				Policies: []string{"foo"},
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "associated with policies or roles",
		},
		{
			name: "invalid roles",
			inputACLToken: &ACLToken{
				Type:  ACLManagementToken,
				Roles: []*ACLTokenRoleLink{{Name: "foo"}},
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "associated with policies or roles",
		},
		{
			name: "name too long",
			inputACLToken: &ACLToken{
				Type: ACLManagementToken,
				Name: uuid.Generate() + uuid.Generate() + uuid.Generate() + uuid.Generate() +
					uuid.Generate() + uuid.Generate() + uuid.Generate() + uuid.Generate(),
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "name too long",
		},
		{
			name: "negative TTL",
			inputACLToken: &ACLToken{
				Type:          ACLManagementToken,
				Name:          "foo",
				ExpirationTTL: -1 * time.Hour,
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "should not be negative",
		},
		{
			name: "TTL too small",
			inputACLToken: &ACLToken{
				Type:           ACLManagementToken,
				Name:           "foo",
				CreateTime:     time.Date(2022, time.July, 11, 16, 23, 0, 0, time.UTC),
				ExpirationTime: pointer.Of(time.Date(2022, time.July, 11, 16, 23, 10, 0, time.UTC)),
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "expiration time cannot be less than",
		},
		{
			name: "TTL too large",
			inputACLToken: &ACLToken{
				Type:           ACLManagementToken,
				Name:           "foo",
				CreateTime:     time.Date(2022, time.July, 11, 16, 23, 0, 0, time.UTC),
				ExpirationTime: pointer.Of(time.Date(2042, time.July, 11, 16, 23, 0, 0, time.UTC)),
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "expiration time cannot be more than",
		},
		{
			name: "valid management",
			inputACLToken: &ACLToken{
				Type: ACLManagementToken,
				Name: "foo",
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "",
		},
		{
			name: "valid client",
			inputACLToken: &ACLToken{
				Type:     ACLClientToken,
				Name:     "foo",
				Policies: []string{"foo"},
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutputError := tc.inputACLToken.Validate(1*time.Minute, 24*time.Hour, tc.inputExistingACLToken)
			if tc.expectedErrorContains != "" {
				require.ErrorContains(t, actualOutputError, tc.expectedErrorContains)
			} else {
				require.NoError(t, actualOutputError)
			}
		})
	}
}

func TestACLToken_HasExpirationTime(t *testing.T) {
	testCases := []struct {
		name           string
		inputACLToken  *ACLToken
		expectedOutput bool ``
	}{
		{
			name:           "nil acl token",
			inputACLToken:  nil,
			expectedOutput: false,
		},
		{
			name:           "default empty value",
			inputACLToken:  &ACLToken{},
			expectedOutput: false,
		},
		{
			name: "expiration set to now",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Now().UTC()),
			},
			expectedOutput: true,
		},
		{
			name: "expiration set to past",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Date(2022, time.February, 21, 19, 35, 0, 0, time.UTC)),
			},
			expectedOutput: true,
		},
		{
			name: "expiration set to future",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Date(2087, time.April, 25, 12, 0, 0, 0, time.UTC)),
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLToken.HasExpirationTime()
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestACLToken_IsExpired(t *testing.T) {
	testCases := []struct {
		name           string
		inputACLToken  *ACLToken
		inputTime      time.Time
		expectedOutput bool
	}{
		{
			name:           "token without expiry",
			inputACLToken:  &ACLToken{},
			inputTime:      time.Now().UTC(),
			expectedOutput: false,
		},
		{
			name:           "empty input time",
			inputACLToken:  &ACLToken{},
			inputTime:      time.Time{},
			expectedOutput: false,
		},
		{
			name: "token not expired",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Date(2022, time.May, 9, 10, 27, 0, 0, time.UTC)),
			},
			inputTime:      time.Date(2022, time.May, 9, 10, 26, 0, 0, time.UTC),
			expectedOutput: false,
		},
		{
			name: "token expired",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Date(2022, time.May, 9, 10, 27, 0, 0, time.UTC)),
			},
			inputTime:      time.Date(2022, time.May, 9, 10, 28, 0, 0, time.UTC),
			expectedOutput: true,
		},
		{
			name: "empty input time",
			inputACLToken: &ACLToken{
				ExpirationTime: pointer.Of(time.Date(2022, time.May, 9, 10, 27, 0, 0, time.UTC)),
			},
			inputTime:      time.Time{},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLToken.IsExpired(tc.inputTime)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestACLToken_HasRoles(t *testing.T) {
	testCases := []struct {
		name           string
		inputToken     *ACLToken
		inputRoleIDs   []string
		expectedOutput bool
	}{
		{
			name: "client token request all subset",
			inputToken: &ACLToken{
				Type: ACLClientToken,
				Roles: []*ACLTokenRoleLink{
					{ID: "foo"},
					{ID: "bar"},
					{ID: "baz"},
				},
			},
			inputRoleIDs:   []string{"foo", "bar", "baz"},
			expectedOutput: true,
		},
		{
			name: "client token request partial subset",
			inputToken: &ACLToken{
				Type: ACLClientToken,
				Roles: []*ACLTokenRoleLink{
					{ID: "foo"},
					{ID: "bar"},
					{ID: "baz"},
				},
			},
			inputRoleIDs:   []string{"foo", "baz"},
			expectedOutput: true,
		},
		{
			name: "client token request one subset",
			inputToken: &ACLToken{
				Type: ACLClientToken,
				Roles: []*ACLTokenRoleLink{
					{ID: "foo"},
					{ID: "bar"},
					{ID: "baz"},
				},
			},
			inputRoleIDs:   []string{"baz"},
			expectedOutput: true,
		},
		{
			name: "client token request no subset",
			inputToken: &ACLToken{
				Type: ACLClientToken,
				Roles: []*ACLTokenRoleLink{
					{ID: "foo"},
					{ID: "bar"},
					{ID: "baz"},
				},
			},
			inputRoleIDs:   []string{"new"},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputToken.HasRoles(tc.inputRoleIDs)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestACLRole_SetHash(t *testing.T) {
	testCases := []struct {
		name           string
		inputACLRole   *ACLRole
		expectedOutput []byte
	}{
		{
			name: "no hash set",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash:        []byte{},
			},
			expectedOutput: []byte{
				122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
				171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
			},
		},
		{
			name: "hash set with change",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					137, 147, 2, 29, 53, 94, 78, 13, 45, 51, 127, 193, 21, 248, 230, 126, 34,
					106, 216, 73, 248, 219, 209, 146, 204, 107, 185, 2, 89, 255, 198, 5,
				},
			},
			expectedOutput: []byte{
				122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
				171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLRole.SetHash()
			require.Equal(t, tc.expectedOutput, actualOutput)
			require.Equal(t, tc.inputACLRole.Hash, actualOutput)
		})
	}
}

func TestACLRole_Validate(t *testing.T) {
	testCases := []struct {
		name                  string
		inputACLRole          *ACLRole
		expectedError         bool
		expectedErrorContains string
	}{
		{
			name: "role name too long",
			inputACLRole: &ACLRole{
				Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectedError:         true,
			expectedErrorContains: "invalid name",
		},
		{
			name: "role name too short",
			inputACLRole: &ACLRole{
				Name: "",
			},
			expectedError:         true,
			expectedErrorContains: "invalid name",
		},
		{
			name: "role name with invalid characters",
			inputACLRole: &ACLRole{
				Name: "--#$%$^%_%%_?>",
			},
			expectedError:         true,
			expectedErrorContains: "invalid name",
		},
		{
			name: "description too long",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectedError:         true,
			expectedErrorContains: "description longer than",
		},
		{
			name: "no policies",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "",
			},
			expectedError:         true,
			expectedErrorContains: "at least one policy should be specified",
		},
		{
			name: "valid",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "",
				Policies: []*ACLRolePolicyLink{
					{Name: "policy-1"},
				},
			},
			expectedError:         false,
			expectedErrorContains: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLRole.Validate()
			if tc.expectedError {
				require.ErrorContains(t, actualOutput, tc.expectedErrorContains)
			} else {
				require.NoError(t, actualOutput)
			}
		})
	}
}

func TestACLRole_Canonicalize(t *testing.T) {
	testCases := []struct {
		name         string
		inputACLRole *ACLRole
	}{
		{
			name:         "no ID set",
			inputACLRole: &ACLRole{},
		},
		{
			name:         "id set",
			inputACLRole: &ACLRole{ID: "some-random-uuid"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			existing := tc.inputACLRole.Copy()
			tc.inputACLRole.Canonicalize()
			if existing.ID == "" {
				require.NotEmpty(t, tc.inputACLRole.ID)
			} else {
				require.Equal(t, existing.ID, tc.inputACLRole.ID)
			}
		})
	}
}

func TestACLRole_Equals(t *testing.T) {
	testCases := []struct {
		name            string
		composedACLRole *ACLRole
		inputACLRole    *ACLRole
		expectedOutput  bool
	}{
		{
			name: "equal with hash set",
			composedACLRole: &ACLRole{
				Name:        "acl-role-",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
			expectedOutput: true,
		},
		{
			name: "equal without hash set",
			composedACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash:        []byte{},
			},
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash:        []byte{},
			},
			expectedOutput: true,
		},
		{
			name:            "both nil",
			composedACLRole: nil,
			inputACLRole:    nil,
			expectedOutput:  true,
		},
		{
			name:            "not equal composed nil",
			composedACLRole: nil,
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
			expectedOutput: false,
		},
		{
			name: "not equal input nil",
			composedACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
			inputACLRole:   nil,
			expectedOutput: false,
		},
		{
			name: "not equal with hash set",
			composedACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					137, 147, 2, 29, 53, 94, 78, 13, 45, 51, 127, 193, 21, 248, 230, 126, 34,
					106, 216, 73, 248, 219, 209, 146, 204, 107, 185, 2, 89, 255, 198, 5,
				},
			},
			expectedOutput: false,
		},
		{
			name: "not equal without hash set",
			composedACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash:        []byte{},
			},
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash:        []byte{},
			},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.composedACLRole.Equal(tc.inputACLRole)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestACLRole_Copy(t *testing.T) {
	testCases := []struct {
		name         string
		inputACLRole *ACLRole
	}{
		{
			name:         "nil input",
			inputACLRole: nil,
		},
		{
			name: "general 1",
			inputACLRole: &ACLRole{
				Name:        "acl-role",
				Description: "mocked-test-acl-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "mocked-test-policy-1"},
					{Name: "mocked-test-policy-2"},
				},
				CreateIndex: 10,
				ModifyIndex: 10,
				Hash: []byte{
					122, 193, 189, 171, 197, 13, 37, 81, 141, 213, 188, 212, 179, 223, 148, 160,
					171, 141, 155, 136, 21, 128, 252, 100, 149, 195, 236, 148, 94, 70, 173, 102,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLRole.Copy()
			require.Equal(t, tc.inputACLRole, actualOutput)
		})
	}
}

func TestACLRole_Stub(t *testing.T) {
	testCases := []struct {
		name           string
		inputACLRole   *ACLRole
		expectedOutput *ACLRoleListStub
	}{
		{
			name: "partially hydrated",
			inputACLRole: &ACLRole{
				ID:          "1d6332c8-02d7-325e-f675-a9bb4aff0c51",
				Name:        "my-lovely-role",
				Description: "",
				Policies: []*ACLRolePolicyLink{
					{Name: "my-lovely-policy"},
				},
				Hash:        []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				CreateIndex: 24,
				ModifyIndex: 24,
			},
			expectedOutput: &ACLRoleListStub{
				ID:          "1d6332c8-02d7-325e-f675-a9bb4aff0c51",
				Name:        "my-lovely-role",
				Description: "",
				Policies: []*ACLRolePolicyLink{
					{Name: "my-lovely-policy"},
				},
				Hash:        []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				CreateIndex: 24,
				ModifyIndex: 24,
			},
		},
		{
			name: "hully hydrated",
			inputACLRole: &ACLRole{
				ID:          "1d6332c8-02d7-325e-f675-a9bb4aff0c51",
				Name:        "my-lovely-role",
				Description: "this-is-my-lovely-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "my-lovely-policy"},
				},
				Hash:        []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				CreateIndex: 24,
				ModifyIndex: 24,
			},
			expectedOutput: &ACLRoleListStub{
				ID:          "1d6332c8-02d7-325e-f675-a9bb4aff0c51",
				Name:        "my-lovely-role",
				Description: "this-is-my-lovely-role",
				Policies: []*ACLRolePolicyLink{
					{Name: "my-lovely-policy"},
				},
				Hash:        []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				CreateIndex: 24,
				ModifyIndex: 24,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputACLRole.Stub()
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func Test_ACLRolesUpsertRequest(t *testing.T) {
	req := ACLRolesUpsertRequest{}
	require.False(t, req.IsRead())
}

func Test_ACLRolesDeleteByIDRequest(t *testing.T) {
	req := ACLRolesDeleteByIDRequest{}
	require.False(t, req.IsRead())
}

func Test_ACLRolesListRequest(t *testing.T) {
	req := ACLRolesListRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLRolesByIDRequest(t *testing.T) {
	req := ACLRolesByIDRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLRoleByIDRequest(t *testing.T) {
	req := ACLRoleByIDRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLRoleByNameRequest(t *testing.T) {
	req := ACLRoleByNameRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLAuthMethodListRequest(t *testing.T) {
	req := ACLAuthMethodListRequest{}
	must.True(t, req.IsRead())
}

func Test_ACLAuthMethodGetRequest(t *testing.T) {
	req := ACLAuthMethodGetRequest{}
	must.True(t, req.IsRead())
}

func TestACLAuthMethodSetHash(t *testing.T) {
	ci.Parallel(t)

	am := &ACLAuthMethod{
		Name: "foo",
		Type: "bad type",
	}
	out1 := am.SetHash()
	must.NotNil(t, out1)
	must.NotNil(t, am.Hash)
	must.Eq(t, out1, am.Hash)

	am.Type = "good type"
	out2 := am.SetHash()
	must.NotNil(t, out2)
	must.NotNil(t, am.Hash)
	must.Eq(t, out2, am.Hash)
	must.NotEq(t, out1, out2)
}

func TestACLAuthMethod_Stub(t *testing.T) {
	ci.Parallel(t)

	maxTokenTTL, _ := time.ParseDuration("3600s")
	am := ACLAuthMethod{
		Name:          fmt.Sprintf("acl-auth-method-%s", uuid.Short()),
		Type:          "acl-auth-mock-type",
		TokenLocality: "locality",
		MaxTokenTTL:   maxTokenTTL,
		Default:       true,
		Config: &ACLAuthMethodConfig{
			OIDCDiscoveryURL:    "http://example.com",
			OIDCClientID:        "mock",
			OIDCClientSecret:    "very secret secret",
			BoundAudiences:      []string{"audience1", "audience2"},
			AllowedRedirectURIs: []string{"foo", "bar"},
			DiscoveryCaPem:      []string{"foo"},
			SigningAlgs:         []string{"bar"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	am.SetHash()

	must.Eq(t, am.Stub(), &ACLAuthMethodStub{
		Name:        am.Name,
		Type:        am.Type,
		Default:     am.Default,
		Hash:        am.Hash,
		CreateIndex: am.CreateIndex,
		ModifyIndex: am.ModifyIndex,
	})

	nilAuthMethod := &ACLAuthMethod{}
	must.Eq(t, nilAuthMethod.Stub(), &ACLAuthMethodStub{})
}

func TestACLAuthMethod_Equal(t *testing.T) {
	ci.Parallel(t)

	maxTokenTTL, _ := time.ParseDuration("3600s")
	am1 := &ACLAuthMethod{
		Name:          fmt.Sprintf("acl-auth-method-%s", uuid.Short()),
		Type:          "acl-auth-mock-type",
		TokenLocality: "locality",
		MaxTokenTTL:   maxTokenTTL,
		Default:       true,
		Config: &ACLAuthMethodConfig{
			OIDCDiscoveryURL:    "http://example.com",
			OIDCClientID:        "mock",
			OIDCClientSecret:    "very secret secret",
			BoundAudiences:      []string{"audience1", "audience2"},
			AllowedRedirectURIs: []string{"foo", "bar"},
			DiscoveryCaPem:      []string{"foo"},
			SigningAlgs:         []string{"bar"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	am1.SetHash()

	// am2 differs from am1 by 1 nested conf field
	am2 := am1.Copy()
	am2.Config.OIDCClientID = "mock2"
	am2.SetHash()

	tests := []struct {
		name    string
		method1 *ACLAuthMethod
		method2 *ACLAuthMethod
		want    bool
	}{
		{"one nil", am1, &ACLAuthMethod{}, false},
		{"both nil", &ACLAuthMethod{}, &ACLAuthMethod{}, true},
		{"one is different than the other", am1, am2, false},
		{"equal", am1, am1.Copy(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method1.Equal(tt.method2)
			must.Eq(t, got, tt.want, must.Sprintf(
				"ACLAuthMethod.Equal() got %v, want %v, test case: %s", got, tt.want, tt.name))
		})
	}
}

func TestACLAuthMethod_Copy(t *testing.T) {
	ci.Parallel(t)

	maxTokenTTL, _ := time.ParseDuration("3600s")
	am1 := &ACLAuthMethod{
		Name:          fmt.Sprintf("acl-auth-method-%s", uuid.Short()),
		Type:          "acl-auth-mock-type",
		TokenLocality: "locality",
		MaxTokenTTL:   maxTokenTTL,
		Default:       true,
		Config: &ACLAuthMethodConfig{
			OIDCDiscoveryURL:    "http://example.com",
			OIDCClientID:        "mock",
			OIDCClientSecret:    "very secret secret",
			BoundAudiences:      []string{"audience1", "audience2"},
			AllowedRedirectURIs: []string{"foo", "bar"},
			DiscoveryCaPem:      []string{"foo"},
			SigningAlgs:         []string{"bar"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	am1.SetHash()

	am2 := am1.Copy()
	am2.SetHash()
	must.Eq(t, am1, am2)

	am3 := am1.Copy()
	am3.Config.AllowedRedirectURIs = []string{"new", "urls"}
	am3.SetHash()
	must.NotEq(t, am1, am3)
}

func TestACLAuthMethod_Validate(t *testing.T) {
	ci.Parallel(t)

	goodTTL, _ := time.ParseDuration("3600s")
	badTTL, _ := time.ParseDuration("3600h")

	tests := []struct {
		name        string
		method      *ACLAuthMethod
		wantErr     bool
		errContains string
	}{
		{
			"valid method",
			&ACLAuthMethod{
				Name:          "mock-auth-method",
				Type:          "OIDC",
				TokenLocality: "local",
				MaxTokenTTL:   goodTTL,
			},
			false,
			"",
		},
		{"invalid name", &ACLAuthMethod{Name: "is this name invalid?"}, true, "invalid name"},
		{"invalid token locality", &ACLAuthMethod{TokenLocality: "regional"}, true, "invalid token locality"},
		{"invalid type", &ACLAuthMethod{Type: "groovy"}, true, "invalid token type"},
		{"invalid max ttl", &ACLAuthMethod{MaxTokenTTL: badTTL}, true, "invalid token type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minTTL, _ := time.ParseDuration("10s")
			maxTTL, _ := time.ParseDuration("10h")
			got := tt.method.Validate(minTTL, maxTTL)
			if tt.wantErr {
				must.Error(t, got, must.Sprintf(
					"ACLAuthMethod.Validate() got error, didn't expect it; test case: %s", tt.name))
				must.StrContains(t, got.Error(), tt.errContains, must.Sprintf(
					"ACLAuthMethod.Validate() got %v error message, expected %v; test case: %s",
					got, tt.errContains, tt.name))
			} else {
				must.NoError(t, got, must.Sprintf(
					"ACLAuthMethod.Validate() expected an error but didn't get one; test case: %s", tt.name))
			}
		})
	}
}

func TestACLAuthMethod_Merge(t *testing.T) {
	ci.Parallel(t)

	name := fmt.Sprintf("acl-auth-method-%s", uuid.Short())

	maxTokenTTL, _ := time.ParseDuration("3600s")
	am1 := &ACLAuthMethod{
		Name:          name,
		TokenLocality: "global",
	}
	am2 := &ACLAuthMethod{
		Name:          name,
		Type:          "OIDC",
		TokenLocality: "locality",
		MaxTokenTTL:   maxTokenTTL,
		Default:       true,
		Config: &ACLAuthMethodConfig{
			OIDCDiscoveryURL:    "http://example.com",
			OIDCClientID:        "mock",
			OIDCClientSecret:    "very secret secret",
			BoundAudiences:      []string{"audience1", "audience2"},
			AllowedRedirectURIs: []string{"foo", "bar"},
			DiscoveryCaPem:      []string{"foo"},
			SigningAlgs:         []string{"bar"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}

	am1.Merge(am2)
	must.Eq(t, am1.TokenLocality, "global")
	minTTL, _ := time.ParseDuration("10s")
	maxTTL, _ := time.ParseDuration("10h")
	must.NoError(t, am1.Validate(minTTL, maxTTL))
}

func TestACLAuthMethodConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	amc1 := &ACLAuthMethodConfig{
		OIDCDiscoveryURL:    "http://example.com",
		OIDCClientID:        "mock",
		OIDCClientSecret:    "very secret secret",
		OIDCScopes:          []string{"groups"},
		BoundAudiences:      []string{"audience1", "audience2"},
		AllowedRedirectURIs: []string{"foo", "bar"},
		DiscoveryCaPem:      []string{"foo"},
		SigningAlgs:         []string{"bar"},
		ClaimMappings:       map[string]string{"foo": "bar"},
		ListClaimMappings:   map[string]string{"foo": "bar"},
	}

	amc2 := amc1.Copy()
	must.Eq(t, amc1, amc2)

	amc3 := amc1.Copy()
	amc3.AllowedRedirectURIs = []string{"new", "urls"}
	must.NotEq(t, amc1, amc3)
}

func TestACLAuthMethod_Canonicalize(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name        string
		inputMethod *ACLAuthMethod
	}{
		{
			"no create time or modify time set",
			&ACLAuthMethod{},
		},
		{
			"create time set to now, modify time not set",
			&ACLAuthMethod{CreateTime: now},
		},
		{
			"both create time and modify time set",
			&ACLAuthMethod{CreateTime: now, ModifyTime: now.Add(time.Hour)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := tt.inputMethod.Copy()
			tt.inputMethod.Canonicalize()
			if existing.CreateTime.IsZero() {
				must.NotEq(t, time.Time{}, tt.inputMethod.CreateTime)
			} else {
				must.Eq(t, existing.CreateTime, tt.inputMethod.CreateTime)
			}
			if existing.ModifyTime.IsZero() {
				must.NotEq(t, time.Time{}, tt.inputMethod.ModifyTime)
			}
		})
	}
}

func TestACLAuthMethod_TokenLocalityIsGlobal(t *testing.T) {
	ci.Parallel(t)

	globalAuthMethod := &ACLAuthMethod{TokenLocality: "global"}
	must.True(t, globalAuthMethod.TokenLocalityIsGlobal())

	localAuthMethod := &ACLAuthMethod{TokenLocality: "local"}
	must.False(t, localAuthMethod.TokenLocalityIsGlobal())
}

func TestACLBindingRule_Canonicalize(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputACLBindingRule *ACLBindingRule
	}{
		{
			name:                "new binding rule",
			inputACLBindingRule: &ACLBindingRule{},
		},
		{
			name: "existing binding rule",
			inputACLBindingRule: &ACLBindingRule{
				ID:         "some-random-uuid",
				CreateTime: time.Now().UTC(),
				ModifyTime: time.Now().UTC(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Make a copy, so we can compare the modified object to it.
			copiedBindingRule := tc.inputACLBindingRule.Copy()

			tc.inputACLBindingRule.Canonicalize()

			if copiedBindingRule.ID == "" {
				must.NotEq(t, "", tc.inputACLBindingRule.ID)
				must.NotEq(t, copiedBindingRule.CreateTime, tc.inputACLBindingRule.CreateTime)
				must.NotEq(t, copiedBindingRule.ModifyTime, tc.inputACLBindingRule.ModifyTime)
			} else {
				must.Eq(t, copiedBindingRule.ID, tc.inputACLBindingRule.ID)
				must.Eq(t, copiedBindingRule.CreateTime, tc.inputACLBindingRule.CreateTime)
				must.NotEq(t, copiedBindingRule.ModifyTime, tc.inputACLBindingRule.ModifyTime)
			}
		})
	}
}

func TestACLBindingRule_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputACLBindingRule *ACLBindingRule
		expectedError       error
	}{
		{
			name: "valid policy type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypePolicy,
				BindName:    "some-policy-name",
			},
			expectedError: nil,
		},
		{
			name: "invalid policy type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypePolicy,
				BindName:    "",
			},
			expectedError: &multierror.Error{
				Errors: []error{
					errors.New("bind name is missing"),
				},
			},
		},
		{
			name: "valid role type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypeRole,
				BindName:    "some-role-name",
			},
			expectedError: nil,
		},
		{
			name: "invalid role type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypeRole,
				BindName:    "",
			},
			expectedError: &multierror.Error{
				Errors: []error{
					errors.New("bind name is missing"),
				},
			},
		},
		{
			name: "valid management type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypeManagement,
				BindName:    "",
			},
			expectedError: nil,
		},
		{
			name: "invalid management type rule",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group_name in list.groups",
				BindType:    ACLBindingRuleBindTypeManagement,
				BindName:    "some-name",
			},
			expectedError: &multierror.Error{
				Errors: []error{
					errors.New("bind name should be empty"),
				},
			},
		},
		{
			name: "invalid selector",
			inputACLBindingRule: &ACLBindingRule{
				Description: "some short description",
				AuthMethod:  "auth0",
				Selector:    "group-name in list.groups",
				BindType:    ACLBindingRuleBindTypePolicy,
				BindName:    "some-policy-name",
			},
			expectedError: &multierror.Error{
				Errors: []error{
					errors.New("selector is invalid: 1:6 (5): no match found, expected: \"!=\", \".\", \"==\", \"[\", [ \\t\\r\\n] or [a-zA-Z0-9_/]"),
				},
			},
		},
		{
			name: "invalid all",
			inputACLBindingRule: &ACLBindingRule{
				Description: uuid.Generate() + uuid.Generate() + uuid.Generate() +
					uuid.Generate() + uuid.Generate() + uuid.Generate() +
					uuid.Generate() + uuid.Generate(),
				AuthMethod: "",
				Selector:   "group-name in list.groups",
				BindType:   "",
				BindName:   "",
			},
			expectedError: &multierror.Error{
				Errors: []error{
					errors.New("auth method is missing"),
					errors.New("description longer than 256"),
					errors.New("bind type is missing"),
					errors.New("selector is invalid: 1:6 (5): no match found, expected: \"!=\", \".\", \"==\", \"[\", [ \\t\\r\\n] or [a-zA-Z0-9_/]"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedError, tc.inputACLBindingRule.Validate())
		})
	}
}

func TestACLBindingRule_Merge(t *testing.T) {
	ci.Parallel(t)

	id := uuid.Short()
	br := &ACLBindingRule{
		ID:          id,
		Description: "old description",
		AuthMethod:  "example-acl-auth-method",
		BindType:    "rule",
		BindName:    "bind name",
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}

	// make a description update
	br_description_update := &ACLBindingRule{
		ID:          id,
		Description: "new description",
	}
	br_description_update.Merge(br)
	must.Eq(t, br_description_update.Description, "new description")
	must.Eq(t, br_description_update.BindType, "rule")
}

func TestACLBindingRule_SetHash(t *testing.T) {
	ci.Parallel(t)

	bindingRule := &ACLBindingRule{
		ID:         uuid.Generate(),
		AuthMethod: "okta",
	}
	out1 := bindingRule.SetHash()
	must.NotNil(t, out1)
	must.NotNil(t, bindingRule.Hash)
	must.Eq(t, out1, bindingRule.Hash)

	bindingRule.Description = "my lovely rule"
	out2 := bindingRule.SetHash()
	must.NotNil(t, out2)
	must.NotNil(t, bindingRule.Hash)
	must.Eq(t, out2, bindingRule.Hash)
	must.NotEq(t, out1, out2)
}

func TestACLBindingRule_Equal(t *testing.T) {
	ci.Parallel(t)

	aclBindingRule1 := &ACLBindingRule{
		ID:          uuid.Short(),
		Description: "mocked-acl-binding-rule",
		AuthMethod:  "auth0",
		Selector:    "engineering in list.roles",
		BindType:    "role-id",
		BindName:    "eng-ro",
		CreateTime:  time.Now().UTC(),
		ModifyTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	aclBindingRule1.SetHash()

	// Create a second binding rule, and modify this from the first.
	aclBindingRule2 := aclBindingRule1.Copy()
	aclBindingRule2.Description = ""
	aclBindingRule2.SetHash()

	testCases := []struct {
		name    string
		method1 *ACLBindingRule
		method2 *ACLBindingRule
		want    bool
	}{
		{"one nil", aclBindingRule1, &ACLBindingRule{}, false},
		{"both nil", &ACLBindingRule{}, &ACLBindingRule{}, true},
		{"not equal", aclBindingRule1, aclBindingRule2, false},
		{"equal", aclBindingRule2, aclBindingRule2.Copy(), true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.method1.Equal(tc.method2)
			must.Eq(t, got, tc.want)
		})
	}
}

func TestACLBindingRule_Copy(t *testing.T) {
	ci.Parallel(t)

	aclBindingRule1 := &ACLBindingRule{
		ID:          uuid.Short(),
		Description: "mocked-acl-binding-rule",
		AuthMethod:  "auth0",
		Selector:    "engineering in list.roles",
		BindType:    "role-id",
		BindName:    "eng-ro",
		CreateTime:  time.Now().UTC(),
		ModifyTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	aclBindingRule1.SetHash()

	aclBindingRule2 := aclBindingRule1.Copy()
	must.Eq(t, aclBindingRule1, aclBindingRule2)

	aclBindingRule3 := aclBindingRule1.Copy()
	aclBindingRule3.Description = ""
	aclBindingRule3.SetHash()
	must.NotEq(t, aclBindingRule1, aclBindingRule3)
}

func TestACLBindingRule_Stub(t *testing.T) {
	ci.Parallel(t)

	aclBindingRule := ACLBindingRule{
		ID:          "some-uuid",
		Description: "my-binding-rule",
		AuthMethod:  "auth0",
		Selector:    "some selector.pattern",
		BindType:    "role",
		BindName:    "some-role-id-or-name",
		CreateTime:  time.Now().UTC(),
		ModifyTime:  time.Now().UTC(),
		CreateIndex: 1309,
		ModifyIndex: 9031,
	}
	aclBindingRule.SetHash()

	must.Eq(t, &ACLBindingRuleListStub{
		ID:          "some-uuid",
		Description: "my-binding-rule",
		AuthMethod:  "auth0",
		Hash:        aclBindingRule.Hash,
		CreateIndex: 1309,
		ModifyIndex: 9031,
	}, aclBindingRule.Stub())
}

func Test_ACLBindingRulesUpsertRequest(t *testing.T) {
	ci.Parallel(t)

	req := ACLBindingRulesUpsertRequest{}
	require.False(t, req.IsRead())
}

func Test_ACLBindingRulesDeleteRequest(t *testing.T) {
	ci.Parallel(t)

	req := ACLBindingRulesDeleteRequest{}
	require.False(t, req.IsRead())
}

func Test_ACLBindingRulesListRequest(t *testing.T) {
	ci.Parallel(t)

	req := ACLBindingRulesListRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLBindingRulesRequest(t *testing.T) {
	ci.Parallel(t)

	req := ACLBindingRulesRequest{}
	require.True(t, req.IsRead())
}

func Test_ACLBindingRuleRequest(t *testing.T) {
	ci.Parallel(t)

	req := ACLBindingRuleRequest{}
	require.True(t, req.IsRead())
}

func TestACLOIDCAuthURLRequest(t *testing.T) {
	ci.Parallel(t)

	req := &ACLOIDCAuthURLRequest{}
	must.False(t, req.IsRead())
}

func TestACLOIDCAuthURLRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testRequest := &ACLOIDCAuthURLRequest{}
	err := testRequest.Validate()
	must.Error(t, err)
	must.StrContains(t, err.Error(), "missing auth method name")
	must.StrContains(t, err.Error(), "missing client nonce")
	must.StrContains(t, err.Error(), "missing redirect URI")
}

func TestACLOIDCCompleteAuthRequest(t *testing.T) {
	ci.Parallel(t)

	req := &ACLOIDCCompleteAuthRequest{}
	must.False(t, req.IsRead())
}

func TestACLOIDCCompleteAuthRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testRequest := &ACLOIDCCompleteAuthRequest{}
	err := testRequest.Validate()
	must.Error(t, err)
	must.StrContains(t, err.Error(), "missing auth method name")
	must.StrContains(t, err.Error(), "missing client nonce")
	must.StrContains(t, err.Error(), "missing state")
	must.StrContains(t, err.Error(), "missing code")
	must.StrContains(t, err.Error(), "missing redirect URI")
}
