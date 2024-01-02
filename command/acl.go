// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ACLCommand struct {
	Meta
}

func (f *ACLCommand) Help() string {
	helpText := `
Usage: nomad acl <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL policies and tokens.
  Users can bootstrap Nomad's ACL system, create policies that restrict access,
  and generate tokens from those policies.

  Bootstrap ACLs:

      $ nomad acl bootstrap

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *ACLCommand) Synopsis() string {
	return "Interact with ACL policies and tokens"
}

func (f *ACLCommand) Name() string { return "acl" }

func (f *ACLCommand) Run(args []string) int {
	return cli.RunResultHelp
}
