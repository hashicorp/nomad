package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
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
			name: "missing policies",
			inputACLToken: &ACLToken{
				Type: ACLClientToken,
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "missing policies",
		},
		{
			name: "invalid policies",
			inputACLToken: &ACLToken{
				Type:     ACLManagementToken,
				Policies: []string{"foo"},
			},
			inputExistingACLToken: nil,
			expectedErrorContains: "associated with policies",
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
