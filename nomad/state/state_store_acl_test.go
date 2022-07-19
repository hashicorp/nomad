package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestStateStore_ACLTokensByExpired(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// This function provides an easy way to get all tokens out of the
	// iterator.
	fromIteratorFunc := func(iter memdb.ResultIterator) []*structs.ACLToken {
		var tokens []*structs.ACLToken
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			tokens = append(tokens, raw.(*structs.ACLToken))
		}
		return tokens
	}

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

	iter, err := testState.ACLTokensByExpired(true)
	require.NoError(t, err)
	tokens := fromIteratorFunc(iter)
	require.Len(t, tokens, 0)

	iter, err = testState.ACLTokensByExpired(false)
	require.NoError(t, err)
	tokens = fromIteratorFunc(iter)
	require.Len(t, tokens, 0)

	// Generate, upsert, and test an expired local token. This token expired
	// long ago and therefore before all others coming in the tests. It should
	// therefore always be the first out.
	expiredLocalToken := mock.ACLToken()
	expiredLocalToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-48 * time.Hour))

	err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{expiredLocalToken})
	require.NoError(t, err)

	iter, err = testState.ACLTokensByExpired(false)
	require.NoError(t, err)
	tokens = fromIteratorFunc(iter)
	require.Len(t, tokens, 1)
	require.Equal(t, expiredLocalToken.AccessorID, tokens[0].AccessorID)

	// Generate, upsert, and test an expired global token. This token expired
	// long ago and therefore before all others coming in the tests. It should
	// therefore always be the first out.
	expiredGlobalToken := mock.ACLToken()
	expiredGlobalToken.Global = true
	expiredGlobalToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-48 * time.Hour))

	err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 30, []*structs.ACLToken{expiredGlobalToken})
	require.NoError(t, err)

	iter, err = testState.ACLTokensByExpired(true)
	require.NoError(t, err)
	tokens = fromIteratorFunc(iter)
	require.Len(t, tokens, 1)
	require.Equal(t, expiredGlobalToken.AccessorID, tokens[0].AccessorID)

	// This test function allows us to run the same test for local and global
	// tokens.
	testFn := func(oldToken *structs.ACLToken, global bool) {

		// Track all the expected expired ACL tokens, including the long
		// expired token.
		var expiredTokens []*structs.ACLToken
		expiredTokens = append(expiredTokens, oldToken)

		// Generate and upsert a number of mixed expired, non-expired tokens.
		mixedTokens := make([]*structs.ACLToken, 20)
		for i := 0; i < 20; i++ {
			mockedToken := mock.ACLToken()
			mockedToken.Global = global
			if i%2 == 0 {
				expiredTokens = append(expiredTokens, mockedToken)
				mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-24 * time.Hour))
			} else {
				mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(24 * time.Hour))
			}
			mixedTokens[i] = mockedToken
		}

		err = testState.UpsertACLTokens(structs.MsgTypeTestSetup, 40, mixedTokens)
		require.NoError(t, err)

		// Check the full listing works as expected as the first 11 elements
		// should all be our expired tokens. Ensure our oldest expired token is
		// first in the list.
		iter, err = testState.ACLTokensByExpired(global)
		require.NoError(t, err)
		tokens = fromIteratorFunc(iter)
		require.ElementsMatch(t, expiredTokens, tokens[:11])
		require.Equal(t, tokens[0], oldToken)
	}

	testFn(expiredLocalToken, false)
	testFn(expiredGlobalToken, true)
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
