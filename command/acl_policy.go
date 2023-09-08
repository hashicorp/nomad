// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ACLPolicyCommand struct {
	Meta
}

func (f *ACLPolicyCommand) Help() string {
	helpText := `
Usage: nomad acl policy <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL policies. Nomad's ACL
  system can be used to control access to data and APIs. ACL policies allow a
  set of capabilities or actions to be granted or allowlisted. For a full guide
  see: https://www.nomadproject.io/guides/acl.html

  Create an ACL policy:

      $ nomad acl policy apply <name> <policy-file>

  List ACL policies:

      $ nomad acl policy list

  Inspect an ACL policy:

      $ nomad acl policy info <policy>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *ACLPolicyCommand) Synopsis() string {
	return "Interact with ACL policies"
}

func (f *ACLPolicyCommand) Name() string { return "acl policy" }

func (f *ACLPolicyCommand) Run(args []string) int {
	return cli.RunResultHelp
}
