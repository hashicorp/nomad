package state

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestStateStore_ACLTokensByExpired(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// This time is the threshold for all expiry calls to be based on. All
	// tokens with expiry can use this as their base and use Add().
	expiryTimeThreshold := time.Date(2022, time.April, 27, 14, 50, 0, 0, time.UTC)

	// Generate two tokens without an expiry time. These tokens should never
	// show up in calls to ACLTokensByExpired.
	neverExpireLocalToken := mock.ACLToken()
	neverExpireGlobalToken := mock.ACLToken()
	neverExpireLocalToken.Global = true

	// Upsert the tokens into state and perform a global and local read of
	// the state.
	err := testState.UpsertACLTokens(structs.MsgTypeTestSetup, 10, []*structs.ACLToken{
		neverExpireLocalToken, neverExpireGlobalToken})
	require.NoError(t, err)

	ids, err := testState.ACLTokensByExpired(true, expiryTimeThreshold, 10)
	require.NoError(t, err)
	require.Len(t, ids, 0)

	ids, err = testState.ACLTokensByExpired(false, expiryTimeThreshold, 10)
	require.NoError(t, err)
	require.Len(t, ids, 0)

	// Generate, upsert, and test an expired local token. This token expired
	// long ago and therefore before all others coming in the tests. It should
	// therefore always be the first out.
	expiredLocalToken := mock.ACLToken()
	expiredLocalToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-48 * time.Hour))

	err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{expiredLocalToken})
	require.NoError(t, err)

	ids, err = testState.ACLTokensByExpired(false, expiryTimeThreshold, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, expiredLocalToken.AccessorID, ids[0])

	// Generate, upsert, and test an expired global token. This token expired
	// long ago and therefore before all others coming in the tests. It should
	// therefore always be the first out.
	expiredGlobalToken := mock.ACLToken()
	expiredGlobalToken.Global = true
	expiredGlobalToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-48 * time.Hour))

	err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 30, []*structs.ACLToken{expiredGlobalToken})
	require.NoError(t, err)

	ids, err = testState.ACLTokensByExpired(true, expiryTimeThreshold, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, expiredGlobalToken.AccessorID, ids[0])

	// This test function allows us to run the same test for local and global
	// tokens.
	testFn := func(oldID string, global bool) {

		// Track all the expected expired accessor IDs including the long
		// expired token.
		var expiredLocalAccessorIDs []string
		expiredLocalAccessorIDs = append(expiredLocalAccessorIDs, oldID)

		// Generate and upsert a number of mixed expired, non-expired local tokens.
		mixedLocalTokens := make([]*structs.ACLToken, 20)
		for i := 0; i < 20; i++ {
			mockedToken := mock.ACLToken()
			mockedToken.Global = global
			if i%2 == 0 {
				expiredLocalAccessorIDs = append(expiredLocalAccessorIDs, mockedToken.AccessorID)
				mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-24 * time.Hour))
			} else {
				mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(24 * time.Hour))
			}
			mixedLocalTokens[i] = mockedToken
		}

		err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 40, mixedLocalTokens)
		require.NoError(t, err)

		// Use a max value higher than the number we have to check the full listing
		// works as expected. Ensure our oldest expired token is first in the list.
		ids, err = testState.ACLTokensByExpired(global, expiryTimeThreshold, 100)
		require.NoError(t, err)
		require.ElementsMatch(t, ids, expiredLocalAccessorIDs)
		require.Equal(t, ids[0], oldID)

		// Use a lower max value than the number of known expired tokens to ensure
		// this is working.
		ids, err = testState.ACLTokensByExpired(global, expiryTimeThreshold, 3)
		require.NoError(t, err)
		require.Len(t, ids, 3)
		require.Equal(t, ids[0], oldID)
	}

	testFn(expiredLocalToken.AccessorID, false)
	testFn(expiredGlobalToken.AccessorID, true)
}

func Test_expiresIndexName(t *testing.T) {
	testCases := []struct {
		globalInput    bool
		expectedOutput string
		name           string
	}{
		{
			globalInput:    false,
			expectedOutput: indexExpiresLocal,
			name:           "local",
		},
		{
			globalInput:    true,
			expectedOutput: indexExpiresGlobal,
			name:           "global",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := expiresIndexName(tc.globalInput)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}
