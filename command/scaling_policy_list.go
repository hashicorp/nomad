package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ScalingPolicyListCommand satisfies the cli.Command interface.
var _ cli.Command = &ScalingPolicyListCommand{}

// ScalingPolicyListCommand implements cli.Command.
type ScalingPolicyListCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ScalingPolicyListCommand) Help() string {
	helpText := `
Usage: nomad scaling policy list [options]

  List is used to list the currently configured scaling policies.

General Options:

  ` + generalOptionsUsage() + `

Policy Info Options:

  -json
    Output the scaling policy in its JSON format.

  -t
    Format and display the scaling policy using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ScalingPolicyListCommand) Synopsis() string {
	return "Display all Nomad scaling policies"
}

func (s *ScalingPolicyListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (s *ScalingPolicyListCommand) Name() string { return "scaling policy list" }

// Run satisfies the cli.Command Run function.
func (s *ScalingPolicyListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		s.Ui.Error("This command takes no arguments")
		s.Ui.Error(commandErrorText(s))
	}

	// Get the HTTP client.
	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	policies, _, err := client.Scaling().ListPolicies(nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error listing scaling policies: %s", err))
		return 1
	}

	if len(policies) == 0 {
		s.Ui.Output("No policies found")
		return 0
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, policies)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		s.Ui.Output(out)
		return 0
	}

	// Create the output table header.
	output := []string{"ID|Enabled|Target"}

	// Sort the list of policies based on their target.
	sortedPolicies := scalingPolicyStubList{policies: policies}
	sort.Sort(sortedPolicies)

	// Iterate the policies and add to the output.
	for _, policy := range sortedPolicies.policies {
		output = append(output, fmt.Sprintf(
			"%s|%v|%s",
			policy.ID, policy.Enabled, formatScalingPolicyTarget(policy.Target)))
	}

	// Output.
	s.Ui.Output(formatList(output))
	return 0
}

// scalingPolicyStubList is a wrapper around []*api.ScalingPolicyListStub that
// list us sort the policies alphabetically based on their target.
type scalingPolicyStubList struct {
	policies []*api.ScalingPolicyListStub
}

// Len satisfies the Len function of the sort.Interface interface.
func (s scalingPolicyStubList) Len() int { return len(s.policies) }

// Swap satisfies the Swap function of the sort.Interface interface.
func (s scalingPolicyStubList) Swap(i, j int) {
	s.policies[i], s.policies[j] = s.policies[j], s.policies[i]
}

// Less satisfies the Less function of the sort.Interface interface.
func (s scalingPolicyStubList) Less(i, j int) bool {

	iTarget := formatScalingPolicyTarget(s.policies[i].Target)
	jTarget := formatScalingPolicyTarget(s.policies[j].Target)

	stringList := []string{iTarget, jTarget}
	sort.Strings(stringList)

	if stringList[0] == iTarget {
		return true
	}
	return false
}
