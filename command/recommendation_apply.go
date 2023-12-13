// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure RecommendationApplyCommand satisfies the cli.Command interface.
var _ cli.Command = &RecommendationApplyCommand{}

// RecommendationApplyCommand implements cli.Command.
type RecommendationApplyCommand struct {
	RecommendationAutocompleteCommand
}

// Help satisfies the cli.Command Help function.
func (r *RecommendationApplyCommand) Help() string {
	helpText := `
Usage: nomad recommendation apply [options] <recommendation_ids>

  Apply one or more Nomad recommendations.

  When ACLs are enabled, this command requires a token with the 'submit-job',
  'read-job', and 'submit-recommendation' capabilities for the
  recommendation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Recommendation Apply Options:

  -detach
    Return immediately instead of entering monitor mode. After applying a
    recommendation, the evaluation ID will be printed to the screen, which can
    be used to examine the evaluation using the eval-status command. If applying
    recommendations for multiple jobs, this value will always be true.

  -policy-override
    If set, any soft mandatory Sentinel policies will be overridden. This allows
    a recommendation to be applied when it would be denied by a policy.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (r *RecommendationApplyCommand) Synopsis() string {
	return "Apply one or more Nomad recommendations"
}

func (r *RecommendationApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(r.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":          complete.PredictNothing,
			"-policy-override": complete.PredictNothing,
			"-verbose":         complete.PredictNothing,
		})
}

// Name returns the name of this command.
func (r *RecommendationApplyCommand) Name() string { return "recommendation apply" }

// Run satisfies the cli.Command Run function.
func (r *RecommendationApplyCommand) Run(args []string) int {
	var detach, override, verbose bool

	flags := r.Meta.FlagSet(r.Name(), FlagSetClient)
	flags.Usage = func() { r.Ui.Output(r.Help()) }
	flags.BoolVar(&override, "policy-override", false, "")
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) < 1 {
		r.Ui.Error("This command takes at least one argument: <recommendation_id>")
		r.Ui.Error(commandErrorText(r))
		return 1
	}

	// Get the HTTP client.
	client, err := r.Meta.Client()
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Create a list of recommendations to apply.
	ids := make([]string, len(args))
	copy(ids, args)

	resp, _, err := client.Recommendations().Apply(ids, override)
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error applying recommendations: %v", err))
		return 1
	}

	// If we should detach, or must because we applied multiple recommendations
	// resulting in more than a single eval to monitor.
	if detach || len(resp.Errors) > 0 || len(resp.UpdatedJobs) > 1 {

		// If we had apply errors, output these at the top so they are easy to
		// find. Always output the heading, this provides some consistency,
		// even if just to show there are no errors.
		r.Ui.Output(r.Colorize().Color("[bold]Errors[reset]"))
		if len(resp.Errors) > 0 {
			r.outputApplyErrors(resp.Errors)
		} else {
			r.Ui.Output("None\n")
		}

		// If we had apply results, output these.
		if len(resp.UpdatedJobs) > 0 {
			r.outputApplyResult(resp.UpdatedJobs)
		}
		return 0
	}

	// When would we ever reach this case? Probably never, but catch this just
	// in case.
	if len(resp.UpdatedJobs) < 1 {
		return 0
	}

	// If we reached here, we should have a single entry to interrogate and
	// monitor.
	length := shortId
	if verbose {
		length = fullId
	}
	mon := newMonitor(r.Ui, client, length)
	return mon.monitor(resp.UpdatedJobs[0].EvalID)
}

func (r *RecommendationApplyCommand) outputApplyErrors(errs []*api.SingleRecommendationApplyError) {
	output := []string{"IDs|Job ID|Error"}
	for _, err := range errs {
		output = append(output, fmt.Sprintf("%s|%s|%s", err.Recommendations, err.JobID, err.Error))
	}
	r.Ui.Output(formatList(output))
	r.Ui.Output("\n")
}

func (r *RecommendationApplyCommand) outputApplyResult(res []*api.SingleRecommendationApplyResult) {
	output := []string{"IDs|Namespace|Job ID|Eval ID|Warnings"}
	for _, r := range res {
		output = append(output, fmt.Sprintf(
			"%s|%s|%s|%s|%s",
			strings.Join(r.Recommendations, ","), r.Namespace, r.JobID, r.EvalID, r.Warnings))
	}
	r.Ui.Output(r.Colorize().Color("[bold]Results[reset]"))
	r.Ui.Output(formatList(output))
}
