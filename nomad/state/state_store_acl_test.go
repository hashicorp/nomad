package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
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

func TestStateStore_UpsertACLRoles(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a mocked ACL role for testing and attempt to upsert this
	// straight into state. It should fail because the ACL policies do not
	// exist.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole()}
	err := testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles)
	require.ErrorContains(t, err, "policy not found")

	// Create the policies our ACL roles wants to link to and then try the
	// upsert again.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 20, mockedACLRoles))

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 20, initialIndex)

	// List all the ACL roles in the table, so we can perform a number of tests
	// on the return array.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLRoles(ws)
	require.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	var count int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		aclRole := raw.(*structs.ACLRole)
		must.Eq(t, 20, aclRole.CreateIndex)
		must.Eq(t, 20, aclRole.ModifyIndex)
	}
	require.Equal(t, 1, count, "incorrect number of ACL roles found")

	// Try writing the same ACL roles to state which should not result in an
	// update to the table index.
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 30, mockedACLRoles))
	reInsertActualIndex, err := testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 20, reInsertActualIndex)

	// Make a change to one of the ACL roles and ensure this update is accepted
	// and the table index is updated.
	updatedMockedACLRole := mockedACLRoles[0].Copy()
	updatedMockedACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: "mocked-test-policy-1"}}
	updatedMockedACLRole.SetHash()
	require.NoError(t, testState.UpsertACLRoles(
		structs.MsgTypeTestSetup, 30, []*structs.ACLRole{updatedMockedACLRole}))

	// Check that the index for the table was modified as expected.
	updatedIndex, err := testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 30, updatedIndex)

	// List the ACL roles in state.
	iter, err = testState.GetACLRoles(ws)
	require.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	count = 0

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		aclRole := raw.(*structs.ACLRole)
		must.Eq(t, 20, aclRole.CreateIndex)
		must.Eq(t, 30, aclRole.ModifyIndex)
	}
	require.Equal(t, 1, count, "incorrect number of ACL roles found")
}

func TestStateStore_ValidateACLRolePolicyLinks(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create our mocked role which includes two ACL policy links.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole()}

	// This should error as no policies exist within state.
	err := testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles)
	require.ErrorContains(t, err, "ACL policy not found")

	// Upsert one ACL policy and retry the role which should still fail.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"

	require.NoError(t, testState.UpsertACLPolicies(structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1}))
	err = testState.UpsertACLRoles(structs.MsgTypeTestSetup, 20, mockedACLRoles)
	require.ErrorContains(t, err, "ACL policy not found")

	// Upsert the second ACL policy. The ACL role should now upsert into state
	// without error.
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(structs.MsgTypeTestSetup, 20, []*structs.ACLPolicy{policy2}))
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 30, mockedACLRoles))
}

func TestStateStore_DeleteACLRolesByID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate a some mocked ACL roles for testing and upsert these straight
	// into state.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles))

	// Try and delete a role using a name that doesn't exist. This should
	// return an error and not change the index for the table.
	err := testState.DeleteACLRolesByID(structs.MsgTypeTestSetup, 20, []string{"not-a-role"})
	require.ErrorContains(t, err, "ACL role not found")

	tableIndex, err := testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 10, tableIndex)

	// Delete one of the previously upserted ACL roles. This should succeed
	// and modify the table index.
	err = testState.DeleteACLRolesByID(structs.MsgTypeTestSetup, 20, []string{mockedACLRoles[0].ID})
	require.NoError(t, err)

	tableIndex, err = testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 20, tableIndex)

	// List the ACL roles and ensure we now only have one present and that it
	// is the one we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLRoles(ws)
	require.NoError(t, err)

	var aclRoles []*structs.ACLRole

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclRoles = append(aclRoles, raw.(*structs.ACLRole))
	}

	require.Len(t, aclRoles, 1, "incorrect number of ACL roles found")
	require.True(t, aclRoles[0].Equals(mockedACLRoles[1]))

	// Delete the final remaining ACL role. This should succeed and modify the
	// table index.
	err = testState.DeleteACLRolesByID(structs.MsgTypeTestSetup, 30, []string{mockedACLRoles[1].ID})
	require.NoError(t, err)

	tableIndex, err = testState.Index(TableACLRoles)
	require.NoError(t, err)
	must.Eq(t, 30, tableIndex)

	// List the ACL roles and ensure we have zero entries.
	iter, err = testState.GetACLRoles(ws)
	require.NoError(t, err)

	aclRoles = []*structs.ACLRole{}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclRoles = append(aclRoles, raw.(*structs.ACLRole))
	}
	require.Len(t, aclRoles, 0, "incorrect number of ACL roles found")
}

func TestStateStore_GetACLRoles(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate a some mocked ACL roles for testing and upsert these straight
	// into state.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles))

	// List the ACL roles and ensure they are exactly as we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLRoles(ws)
	require.NoError(t, err)

	var aclRoles []*structs.ACLRole

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclRoles = append(aclRoles, raw.(*structs.ACLRole))
	}

	expected := mockedACLRoles
	for i := range expected {
		expected[i].CreateIndex = 10
		expected[i].ModifyIndex = 10
	}

	require.ElementsMatch(t, aclRoles, expected)
}

func TestStateStore_GetACLRoleByID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate a some mocked ACL roles for testing and upsert these straight
	// into state.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles))

	ws := memdb.NewWatchSet()

	// Try reading an ACL role that does not exist.
	aclRole, err := testState.GetACLRoleByID(ws, "not-a-role")
	require.NoError(t, err)
	require.Nil(t, aclRole)

	// Read the two ACL roles that we should find.
	aclRole, err = testState.GetACLRoleByID(ws, mockedACLRoles[0].ID)
	require.NoError(t, err)
	require.Equal(t, mockedACLRoles[0], aclRole)

	aclRole, err = testState.GetACLRoleByID(ws, mockedACLRoles[1].ID)
	require.NoError(t, err)
	require.Equal(t, mockedACLRoles[1], aclRole)
}

func TestStateStore_GetACLRoleByName(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate a some mocked ACL roles for testing and upsert these straight
	// into state.
	mockedACLRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, mockedACLRoles))

	ws := memdb.NewWatchSet()

	// Try reading an ACL role that does not exist.
	aclRole, err := testState.GetACLRoleByName(ws, "not-a-role")
	require.NoError(t, err)
	require.Nil(t, aclRole)

	// Read the two ACL roles that we should find.
	aclRole, err = testState.GetACLRoleByName(ws, mockedACLRoles[0].Name)
	require.NoError(t, err)
	require.Equal(t, mockedACLRoles[0], aclRole)

	aclRole, err = testState.GetACLRoleByName(ws, mockedACLRoles[1].Name)
	require.NoError(t, err)
	require.Equal(t, mockedACLRoles[1], aclRole)
}
