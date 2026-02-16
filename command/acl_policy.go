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

type ACLPolicyCommand struct {
	Meta
}

func (f *ACLPolicyCommand) Help() string {
	helpText := `
Usage: nomad acl policy <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL policies. Nomad's ACL
  system can be used to control access to data and APIs. ACL policies allow a
  set of capabilities or actions to be granted or allowlisted. For a full guide
  see: https://developer.hashicorp.com/nomad/docs/secure/acl

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

// ACLPolicyPredictor returns an autocomplete predictor that can be used
// across multiple binding rule commands
func ACLPolicyPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		policies, _, err := client.ACLPolicies().List(&api.QueryOptions{
			Prefix: a.Last,
		})
		if err != nil {
			return []string{}
		}

		return helper.ConvertSlice(policies,
			func(p *api.ACLPolicyListStub) string { return p.Name })
	})
}
