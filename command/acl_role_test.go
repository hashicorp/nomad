// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/require"
)

func Test_formatACLRole(t *testing.T) {
	inputACLRole := api.ACLRole{
		ID:          "this-is-usually-a-uuid",
		Name:        "this-is-my-friendly-name",
		Description: "this-is-my-friendly-name",
		Policies: []*api.ACLRolePolicyLink{
			{Name: "policy-link-1"},
			{Name: "policy-link-2"},
			{Name: "policy-link-3"},
			{Name: "policy-link-4"},
		},
		CreateIndex: 13,
		ModifyIndex: 1313,
	}
	expectedOutput := "ID           = this-is-usually-a-uuid\nName         = this-is-my-friendly-name\nDescription  = this-is-my-friendly-name\nPolicies     = policy-link-1,policy-link-2,policy-link-3,policy-link-4\nCreate Index = 13\nModify Index = 1313"
	actualOutput := formatACLRole(&inputACLRole)
	require.Equal(t, expectedOutput, actualOutput)
}

func Test_aclRolePolicyLinkToStringList(t *testing.T) {
	inputPolicyLinks := []*api.ACLRolePolicyLink{
		{Name: "z-policy-link-1"},
		{Name: "a-policy-link-2"},
		{Name: "policy-link-3"},
		{Name: "b-policy-link-4"},
	}
	expectedOutput := []string{
		"a-policy-link-2",
		"b-policy-link-4",
		"policy-link-3",
		"z-policy-link-1",
	}
	actualOutput := aclRolePolicyLinkToStringList(inputPolicyLinks)
	require.Equal(t, expectedOutput, actualOutput)
}

func Test_aclRolePolicyNamesToPolicyLinks(t *testing.T) {
	inputPolicyNames := []string{
		"policy-link-1", "policy-link-2", "policy-link-3", "policy-link-4",
		"policy-link-1", "policy-link-2", "policy-link-3", "policy-link-4",
		"policy-link-1", "policy-link-2", "policy-link-3", "policy-link-4",
		"policy-link-1", "policy-link-2", "policy-link-3", "policy-link-4",
	}
	expectedOutput := []*api.ACLRolePolicyLink{
		{Name: "policy-link-1"},
		{Name: "policy-link-2"},
		{Name: "policy-link-3"},
		{Name: "policy-link-4"},
	}
	actualOutput := aclRolePolicyNamesToPolicyLinks(inputPolicyNames)
	require.ElementsMatch(t, expectedOutput, actualOutput)
}
