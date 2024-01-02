// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_UpsertACLBindingRules(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a mocked ACL binding rule for testing and attempt to upsert
	// this straight into state. It should fail because the auth method does
	// not exist.
	mockedACLBindingRules := []*structs.ACLBindingRule{mock.ACLBindingRule()}
	err := testState.UpsertACLBindingRules(10, mockedACLBindingRules, false)
	must.EqError(t, err, "ACL binding rule insert failed: ACL auth method not found")

	// Create an auth method and ensure the binding rule is updated, so it is
	// related to it.
	authMethod := mock.ACLOIDCAuthMethod()
	mockedACLBindingRules[0].AuthMethod = authMethod.Name

	must.NoError(t, testState.UpsertACLAuthMethods(10, []*structs.ACLAuthMethod{authMethod}))
	must.NoError(t, testState.UpsertACLBindingRules(20, mockedACLBindingRules, false))

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 20, initialIndex)

	// List all the ACL binding rules in the table, so we can perform a number
	// of tests on the return array.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLBindingRules(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	var count int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		aclRole := raw.(*structs.ACLBindingRule)
		must.Eq(t, 20, aclRole.CreateIndex)
		must.Eq(t, 20, aclRole.ModifyIndex)
	}
	must.Eq(t, 1, count)

	// Try writing the same ACL binding rule to state which should not result
	// in an update to the table index.
	must.NoError(t, testState.UpsertACLBindingRules(20, mockedACLBindingRules, false))
	reInsertActualIndex, err := testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 20, reInsertActualIndex)

	// Make a change to the binding rule and ensure this update is accepted and
	// the table index is updated.
	updatedMockedACLBindingRule := mockedACLBindingRules[0].Copy()
	updatedMockedACLBindingRule.BindType = "role-name"
	updatedMockedACLBindingRule.SetHash()
	must.NoError(t, testState.UpsertACLBindingRules(
		30, []*structs.ACLBindingRule{updatedMockedACLBindingRule}, false))

	// Check that the index for the table was modified as expected.
	updatedIndex, err := testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 30, updatedIndex)

	// List the ACL roles in state.
	iter, err = testState.GetACLBindingRules(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	count = 0

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		aclRole := raw.(*structs.ACLBindingRule)
		must.Eq(t, 20, aclRole.CreateIndex)
		must.Eq(t, 30, aclRole.ModifyIndex)
	}
	must.Eq(t, 1, count)

	// Now try inserting an ACL binding rule using the missing auth methods
	// argument to simulate replication.
	replicatedACLBindingRule := []*structs.ACLBindingRule{mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(40, replicatedACLBindingRule, true))

	replicatedACLBindingRuleResp, err := testState.GetACLBindingRule(ws, replicatedACLBindingRule[0].ID)
	must.NoError(t, err)
	must.Eq(t, replicatedACLBindingRule[0].Hash, replicatedACLBindingRuleResp.Hash)
}

func TestStateStore_DeleteACLBindingRules(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL binding rules for testing and upsert these
	// straight into state.
	mockedACLBindingRoles := []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(10, mockedACLBindingRoles, true))

	// Try and delete a binding rule using an ID that doesn't exist. This
	// should return an error and not change the index for the table.
	err := testState.DeleteACLBindingRules(20, []string{uuid.Generate()})
	must.EqError(t, err, "ACL binding rule not found")

	tableIndex, err := testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 10, tableIndex)

	// Delete one of the previously upserted ACL binding rules. This should
	// succeed and modify the table index.
	must.NoError(t, testState.DeleteACLBindingRules(20, []string{mockedACLBindingRoles[0].ID}))

	tableIndex, err = testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 20, tableIndex)

	// List the ACL binding rules and ensure we now only have one present and
	// that it is the one we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLBindingRules(ws)
	must.NoError(t, err)

	var aclBindingRules []*structs.ACLBindingRule

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRules = append(aclBindingRules, raw.(*structs.ACLBindingRule))
	}

	must.Len(t, 1, aclBindingRules)
	must.True(t, aclBindingRules[0].Equal(mockedACLBindingRoles[1]))

	// Delete the final remaining ACL binding rule. This should succeed and
	// modify the table index.
	must.NoError(t, testState.DeleteACLBindingRules(30, []string{mockedACLBindingRoles[1].ID}))

	tableIndex, err = testState.Index(TableACLBindingRules)
	must.NoError(t, err)
	must.Eq(t, 30, tableIndex)

	// List the ACL binding rules and ensure we have zero entries.
	iter, err = testState.GetACLBindingRules(ws)
	must.NoError(t, err)

	aclBindingRules = make([]*structs.ACLBindingRule, 0)

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRules = append(aclBindingRules, raw.(*structs.ACLBindingRule))
	}
	must.Len(t, 0, aclBindingRules)
}

func TestStateStore_GetACLBindingRules(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL binding rules for testing and upsert these
	// straight into state.
	mockedACLBindingRoles := []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(10, mockedACLBindingRoles, true))

	// List the ACL binding rules and ensure they are exactly as we expect.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetACLBindingRules(ws)
	must.NoError(t, err)

	var aclBindingRules []*structs.ACLBindingRule

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRules = append(aclBindingRules, raw.(*structs.ACLBindingRule))
	}

	expected := mockedACLBindingRoles
	for i := range expected {
		expected[i].CreateIndex = 10
		expected[i].ModifyIndex = 10
	}

	must.SliceContainsAll(t, aclBindingRules, expected)
}

func TestStateStore_GetACLBindingRule(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL binding rules for testing and upsert these
	// straight into state.
	mockedACLBindingRoles := []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(10, mockedACLBindingRoles, true))

	ws := memdb.NewWatchSet()

	// Try reading an ACL binding rule that does not exist.
	aclBindingRule, err := testState.GetACLBindingRule(ws, uuid.Generate())
	must.NoError(t, err)
	must.Nil(t, aclBindingRule)

	// Read the two ACL binding rules that we should find.
	aclBindingRule, err = testState.GetACLBindingRule(ws, mockedACLBindingRoles[0].ID)
	must.NoError(t, err)
	must.Eq(t, mockedACLBindingRoles[0], aclBindingRule)

	aclBindingRule, err = testState.GetACLBindingRule(ws, mockedACLBindingRoles[1].ID)
	must.NoError(t, err)
	must.Eq(t, mockedACLBindingRoles[1], aclBindingRule)
}

func TestStateStore_GetACLBindingRulesByAuthMethod(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate a some mocked ACL binding rules for testing and upsert these
	// straight into state.
	mockedACLBindingRoles := []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(10, mockedACLBindingRoles, true))

	ws := memdb.NewWatchSet()

	// Lookup ACL binding rules using an auth method that is not referenced. We
	// should not get any results within the iterator.
	iter, err := testState.GetACLBindingRulesByAuthMethod(ws, "not-an-auth-method")
	must.NoError(t, err)

	var aclBindingRules []*structs.ACLBindingRule

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRules = append(aclBindingRules, raw.(*structs.ACLBindingRule))
	}
	must.Len(t, 0, aclBindingRules)

	// Lookup ACL binding rules using an auth method that is referenced by both
	// mocked rules. Ensure the results are as expected.
	iter, err = testState.GetACLBindingRulesByAuthMethod(ws, mockedACLBindingRoles[0].AuthMethod)
	must.NoError(t, err)

	aclBindingRules = make([]*structs.ACLBindingRule, 0)

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRules = append(aclBindingRules, raw.(*structs.ACLBindingRule))
	}
	must.Len(t, 2, aclBindingRules)
}
