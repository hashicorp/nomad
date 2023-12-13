// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
)

// Ensure ScalingPolicyInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ScalingPolicyInfoCommand{}

// ScalingPolicyListCommand implements cli.Command.
type ScalingPolicyInfoCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ScalingPolicyInfoCommand) Help() string {
	helpText := `
Usage: nomad scaling policy info [options] <policy_id>

  Info is used to read the specified scaling policy.

  If ACLs are enabled, this command requires a token with the 'read-job' and
  'list-jobs' capabilities for the policy's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Policy Info Options:

  -verbose
    Display full information.

  -json
    Output the scaling policy in its JSON format.

  -t
    Format and display the scaling policy using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ScalingPolicyInfoCommand) Synopsis() string {
	return "Display an individual Nomad scaling policy"
}

func (s *ScalingPolicyInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (s *ScalingPolicyInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := s.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.ScalingPolicies, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.ScalingPolicies]
	})
}

// Name returns the name of this command.
func (s *ScalingPolicyInfoCommand) Name() string { return "scaling policy info" }

// Run satisfies the cli.Command Run function.
func (s *ScalingPolicyInfoCommand) Run(args []string) int {
	var json, verbose bool
	var tmpl string

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Get the HTTP client.
	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	args = flags.Args()

	// Formatted list mode if no policy ID
	if len(args) == 0 && (json || len(tmpl) > 0) {
		policies, _, err := client.Scaling().ListPolicies(nil)
		if err != nil {
			s.Ui.Error(fmt.Sprintf("Error listing scaling policies: %v", err))
			return 1
		}
		out, err := Format(json, tmpl, policies)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		s.Ui.Output(out)
		return 0
	}

	if len(args) != 1 {
		s.Ui.Error("This command takes one of the following argument conditions:")
		s.Ui.Error(" * A single <policy_id>")
		s.Ui.Error(" * No arguments, with output format specified")
		s.Ui.Error(commandErrorText(s))
		return 1
	}
	policyID := args[0]
	if len(policyID) == 1 {
		s.Ui.Error("Identifier must contain at least two characters.")
		return 1
	}

	policyID = sanitizeUUIDPrefix(policyID)
	policies, _, err := client.Scaling().ListPolicies(&api.QueryOptions{
		Prefix: policyID,
	})
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error querying scaling policy: %v", err))
		return 1
	}
	if len(policies) == 0 {
		s.Ui.Error(fmt.Sprintf("No scaling policies with prefix or id %q found", policyID))
		return 1
	}
	if len(policies) > 1 {
		out := formatScalingPolicies(policies, length)
		s.Ui.Error(fmt.Sprintf("Prefix matched multiple scaling policies\n\n%s", out))
		return 0
	}

	policy, _, err := client.Scaling().GetPolicy(policies[0].ID, nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error querying scaling policy: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, policy)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}

		s.Ui.Output(out)
		return 0
	}

	// Format the policy document which is a freeform map[string]interface{}
	// and therefore can only be made pretty to a certain extent. Do this
	// before the rest of the formatting so any errors are clearly passed back
	// to the CLI.
	out := "<empty>"
	if len(policy.Policy) > 0 {
		out, err = Format(true, "", policy.Policy)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
	}

	info := []string{
		fmt.Sprintf("ID|%s", limit(policy.ID, length)),
		fmt.Sprintf("Enabled|%v", *policy.Enabled),
		fmt.Sprintf("Target|%s", formatScalingPolicyTarget(policy.Target)),
		fmt.Sprintf("Min|%v", *policy.Min),
		fmt.Sprintf("Max|%v", *policy.Max),
	}
	s.Ui.Output(formatKV(info))
	s.Ui.Output("\nPolicy:")
	s.Ui.Output(out)
	return 0
}
