package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
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
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (s *ScalingPolicyInfoCommand) Name() string { return "scaling policy info" }

// Run satisfies the cli.Command Run function.
func (s *ScalingPolicyInfoCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) != 1 {
		s.Ui.Error("This command takes one argument: <policy_id>")
		s.Ui.Error(commandErrorText(s))
		return 1
	}

	// Get the policy ID.
	policyID := args[0]

	// Get the HTTP client.
	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	policy, _, err := client.Scaling().GetPolicy(policyID, nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error listing scaling policies: %s", err))
		return 1
	}

	// If the user has specified to output the policy as JSON or using a
	// template then perform this action for the entire object and exit the
	// command.
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
	out, err := Format(true, "", policy.Policy)
	if err != nil {
		s.Ui.Error(err.Error())
		return 1
	}

	info := []string{
		fmt.Sprintf("ID|%s", policy.ID),
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
