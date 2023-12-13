// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

// Ensure ACLBindingRuleCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLBindingRuleCommand{}

// ACLBindingRuleCommand implements cli.Command.
type ACLBindingRuleCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLBindingRuleCommand) Help() string {
	helpText := `
Usage: nomad acl binding-rule <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL binding rules.
  Nomad's ACL system can be used to control access to data and APIs. For a full
  guide see: https://www.nomadproject.io/guides/acl.html

  Create an ACL binding rule:

      $ nomad acl binding-rule create \
          -auth-method=auth0 \
          -selector="nomad-engineering in list.groups" \
          -bind-type=role \
          -bind-name="custer-admin" \

  List all ACL binding rules:

      $ nomad acl binding-rule list

  Lookup a specific ACL binding rule:

      $ nomad acl binding-rule info <acl_binding_rule_id>

  Update an ACL binding rule:

      $ nomad acl binding-rule update \
          -description="nomad engineering team" \
          <acl_binding_rule_id>

  Delete an ACL binding rule:

      $ nomad acl binding-rule delete <acl_binding_rule_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLBindingRuleCommand) Synopsis() string { return "Interact with ACL binding rules" }

// Name returns the name of this command.
func (a *ACLBindingRuleCommand) Name() string { return "acl binding-rule" }

// Run satisfies the cli.Command Run function.
func (a *ACLBindingRuleCommand) Run(_ []string) int { return cli.RunResultHelp }

// formatACLBindingRule formats and converts the ACL binding rule API object
// into a string KV representation suitable for console output.
func formatACLBindingRule(aclBindingRule *api.ACLBindingRule) string {
	return formatKV([]string{
		fmt.Sprintf("ID|%s", aclBindingRule.ID),
		fmt.Sprintf("Description|%s", aclBindingRule.Description),
		fmt.Sprintf("Auth Method|%s", aclBindingRule.AuthMethod),
		fmt.Sprintf("Selector|%q", aclBindingRule.Selector),
		fmt.Sprintf("Bind Type|%s", aclBindingRule.BindType),
		fmt.Sprintf("Bind Name|%s", aclBindingRule.BindName),
		fmt.Sprintf("Create Time|%s", aclBindingRule.CreateTime),
		fmt.Sprintf("Modify Time|%s", aclBindingRule.ModifyTime),
		fmt.Sprintf("Create Index|%d", aclBindingRule.CreateIndex),
		fmt.Sprintf("Modify Index|%d", aclBindingRule.ModifyIndex),
	})
}
