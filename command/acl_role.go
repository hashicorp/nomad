// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

// Ensure ACLRoleCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLRoleCommand{}

// ACLRoleCommand implements cli.Command.
type ACLRoleCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLRoleCommand) Help() string {
	helpText := `
Usage: nomad acl role <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL roles. Nomad's ACL
  system can be used to control access to data and APIs. ACL roles are
  associated with one or more ACL policies which grant specific capabilities.
  For a full guide see: https://www.nomadproject.io/guides/acl.html

  Create an ACL role:

      $ nomad acl role create -name="name" -policy-name="policy-name"

  List all ACL roles:

      $ nomad acl role list

  Lookup a specific ACL role:

      $ nomad acl role info <acl_role_id>

  Update an ACL role:

      $ nomad acl role update -name="updated-name" <acl_role_id>

  Delete an ACL role:

      $ nomad acl role delete <acl_role_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLRoleCommand) Synopsis() string { return "Interact with ACL roles" }

// Name returns the name of this command.
func (a *ACLRoleCommand) Name() string { return "acl role" }

// Run satisfies the cli.Command Run function.
func (a *ACLRoleCommand) Run(_ []string) int { return cli.RunResultHelp }

// formatACLRole formats and converts the ACL role API object into a string KV
// representation suitable for console output.
func formatACLRole(aclRole *api.ACLRole) string {
	return formatKV([]string{
		fmt.Sprintf("ID|%s", aclRole.ID),
		fmt.Sprintf("Name|%s", aclRole.Name),
		fmt.Sprintf("Description|%s", aclRole.Description),
		fmt.Sprintf("Policies|%s", strings.Join(aclRolePolicyLinkToStringList(aclRole.Policies), ",")),
		fmt.Sprintf("Create Index|%d", aclRole.CreateIndex),
		fmt.Sprintf("Modify Index|%d", aclRole.ModifyIndex),
	})
}

// aclRolePolicyLinkToStringList converts an array of ACL role policy links to
// an array of string policy names. The returned array will be sorted.
func aclRolePolicyLinkToStringList(policyLinks []*api.ACLRolePolicyLink) []string {
	policies := make([]string, len(policyLinks))
	for i, policy := range policyLinks {
		policies[i] = policy.Name
	}
	sort.Strings(policies)
	return policies
}

// aclRolePolicyNamesToPolicyLinks takes a list of policy names as a string
// array and converts this to an array of ACL role policy links. Any duplicate
// names are removed.
func aclRolePolicyNamesToPolicyLinks(policyNames []string) []*api.ACLRolePolicyLink {
	var policyLinks []*api.ACLRolePolicyLink
	keys := make(map[string]struct{})

	for _, policyName := range policyNames {
		if _, ok := keys[policyName]; !ok {
			policyLinks = append(policyLinks, &api.ACLRolePolicyLink{Name: policyName})
			keys[policyName] = struct{}{}
		}
	}
	return policyLinks
}
