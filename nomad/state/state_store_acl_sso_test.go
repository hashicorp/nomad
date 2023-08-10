// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_UpsertACLAuthMethods(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Create mock auth methods
	mockedACLAuthMethods := []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}

	must.NoError(t, testState.UpsertACLAuthMethods(10, mockedACLAuthMethods))

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 10, initialIndex)

	// List all the auth methods in the table, so we can perform a number of
	// tests on the return array.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	var count int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		authMethod := raw.(*structs.ACLAuthMethod)
		must.Eq(t, 10, authMethod.CreateIndex)
		must.Eq(t, 10, authMethod.ModifyIndex)
	}
	must.Eq(t, 2, count)

	// Try writing the same auth methods to state which should not result in an
	// update to the table index.
	must.NoError(t, testState.UpsertACLAuthMethods(20, mockedACLAuthMethods))
	reInsertActualIndex, err := testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 10, reInsertActualIndex)

	// Make a change to the auth methods and ensure this update is accepted and
	// the table index is updated.
	updatedMockedAuthMethod1 := mockedACLAuthMethods[0].Copy()
	updatedMockedAuthMethod1.Type = "new type"
	updatedMockedAuthMethod1.SetHash()
	updatedMockedAuthMethod2 := mockedACLAuthMethods[1].Copy()
	updatedMockedAuthMethod2.Type = "yet another new type"
	updatedMockedAuthMethod2.SetHash()
	must.NoError(t, testState.UpsertACLAuthMethods(20, []*structs.ACLAuthMethod{
		updatedMockedAuthMethod1, updatedMockedAuthMethod2,
	}))

	// Check that the index for the table was modified as expected.
	updatedIndex, err := testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 20, updatedIndex)

	// List the ACL auth methods in state.
	iter, err = testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	count = 0

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		aclAuthMethod := raw.(*structs.ACLAuthMethod)
		must.Eq(t, 10, aclAuthMethod.CreateIndex)
		must.Eq(t, 20, aclAuthMethod.ModifyIndex)
	}
	must.Eq(t, 2, count, must.Sprintf("incorrect number of ACL auth methods found"))

	// Try adding a new auth method, which has a name clash with an existing
	// entry.
	dup := mock.ACLOIDCAuthMethod()
	dup.Name = mockedACLAuthMethods[0].Name
	dup.Type = mockedACLAuthMethods[0].Type

	err = testState.UpsertACLAuthMethods(50, []*structs.ACLAuthMethod{dup})
	must.NoError(t, err)

	// Get all the ACL auth methods from state.
	iter, err = testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	count = 0

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++
	}
	must.Eq(t, 2, count, must.Sprintf("incorrect number of ACL auth methods found"))
}

func TestStateStore_DeleteACLAuthMethods(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some mocked ACL auth methods for testing and upsert these
	// straight into state.
	mockedACLAuthMethods := []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}
	must.NoError(t, testState.UpsertACLAuthMethods(10, mockedACLAuthMethods))

	// Try and delete a method using a name that doesn't exist. This should
	// return an error and not change the index for the table.
	err := testState.DeleteACLAuthMethods(20, []string{"not-a-method"})
	must.EqError(t, err, "ACL auth method not found")

	tableIndex, err := testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 10, tableIndex)

	// Delete one of the previously upserted auth methods. This should succeed
	// and modify the table index.
	err = testState.DeleteACLAuthMethods(20, []string{mockedACLAuthMethods[0].Name})
	must.NoError(t, err)

	tableIndex, err = testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 20, tableIndex)

	// List the ACL auth methods and ensure we now only have one present and
	// that it is the one we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	var aclAuthMethods []*structs.ACLAuthMethod

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclAuthMethods = append(aclAuthMethods, raw.(*structs.ACLAuthMethod))
	}

	must.Len(t, 1, aclAuthMethods, must.Sprintf("incorrect number of auth methods found"))
	must.True(t, aclAuthMethods[0].Equal(mockedACLAuthMethods[1]))

	// Delete the final remaining auth method. This should succeed and modify
	// the table index.
	err = testState.DeleteACLAuthMethods(30, []string{mockedACLAuthMethods[1].Name})
	must.NoError(t, err)

	tableIndex, err = testState.Index(TableACLAuthMethods)
	must.NoError(t, err)
	must.Eq(t, 30, tableIndex)

	// List the auth methods and ensure we have zero entries.
	iter, err = testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	aclAuthMethods = []*structs.ACLAuthMethod{}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclAuthMethods = append(aclAuthMethods, raw.(*structs.ACLAuthMethod))
	}
	must.Len(t, 0, aclAuthMethods, must.Sprintf("incorrect number of ACL roles found"))
}

func TestStateStore_GetACLAuthMethods(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL auth methods for testing and upsert these
	// straight into state.
	mockedACLAuthMethods := []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}
	must.NoError(t, testState.UpsertACLAuthMethods(10, mockedACLAuthMethods))

	// List the auth methods and ensure they are exactly as we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLAuthMethods(ws)
	must.NoError(t, err)

	var aclAuthMethods []*structs.ACLAuthMethod

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclAuthMethods = append(aclAuthMethods, raw.(*structs.ACLAuthMethod))
	}

	expected := mockedACLAuthMethods
	for i := range expected {
		expected[i].CreateIndex = 10
		expected[i].ModifyIndex = 10
	}

	must.SliceContainsAll(t, aclAuthMethods, expected)
}

func TestStateStore_GetACLAuthMethodByName(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL auth methods for testing and upsert these
	// straight into state.
	mockedACLAuthMethods := []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}
	must.NoError(t, testState.UpsertACLAuthMethods(10, mockedACLAuthMethods))

	ws := memdb.NewWatchSet()

	// Try reading an auth method that does not exist.
	authMethod, err := testState.GetACLAuthMethodByName(ws, "not-a-method")
	must.NoError(t, err)
	must.Nil(t, authMethod)

	// Read the two ACL roles that we should find.
	authMethod, err = testState.GetACLAuthMethodByName(ws, mockedACLAuthMethods[0].Name)
	must.NoError(t, err)
	must.Equal(t, mockedACLAuthMethods[0], authMethod)

	authMethod, err = testState.GetACLAuthMethodByName(ws, mockedACLAuthMethods[1].Name)
	must.NoError(t, err)
	must.Equal(t, mockedACLAuthMethods[1], authMethod)
}

func TestStateStore_GetDefaultACLAuthMethod(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate 2 auth methods, make one of them default
	am1 := mock.ACLOIDCAuthMethod()
	am1.Default = true
	am2 := mock.ACLOIDCAuthMethod()

	// upsert
	mockedACLAuthMethods := []*structs.ACLAuthMethod{am1, am2}
	must.NoError(t, testState.UpsertACLAuthMethods(10, mockedACLAuthMethods))

	// Get the default method
	ws := memdb.NewWatchSet()
	defaultACLAuthMethod, err := testState.GetDefaultACLAuthMethod(ws)
	must.NoError(t, err)

	must.True(t, defaultACLAuthMethod.Default)
	must.Eq(t, am1, defaultACLAuthMethod)

}
