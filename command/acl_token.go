// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

type ACLTokenCommand struct {
	Meta
}

func (f *ACLTokenCommand) Help() string {
	helpText := `
Usage: nomad acl token <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL tokens. Nomad's ACL
  system can be used to control access to data and APIs. ACL tokens are
  associated with one or more ACL policies which grant specific capabilities.
  For a full guide see: https://developer.hashicorp.com/nomad/docs/secure/acl

  Create an ACL token:

      $ nomad acl token create -name "my-token" -policy foo -policy bar

  Lookup a token and display its associated policies:

      $ nomad acl policy info <token_accessor_id>

  Revoke an ACL token:

      $ nomad acl policy delete <token_accessor_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *ACLTokenCommand) Synopsis() string {
	return "Interact with ACL tokens"
}

func (f *ACLTokenCommand) Name() string { return "acl token" }

func (f *ACLTokenCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// ACLTokenPredictor returns an autocomplete predictor that can be used across
// multiple token commands.
func ACLTokenPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		tokens, _, err := client.ACLTokens().List(&api.QueryOptions{
			Prefix: a.Last,
		})
		if err != nil {
			return []string{}
		}

		return helper.ConvertSlice(tokens,
			func(t *api.ACLTokenListStub) string { return t.AccessorID })
	})
}
