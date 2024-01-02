// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api/contexts"
)

// Ensure RecommendationDismissCommand satisfies the cli.Command interface.
var _ cli.Command = &RecommendationDismissCommand{}

// RecommendationAutocompleteCommand provides AutocompleteArgs for all
// recommendation commands that support prefix-search autocompletion
type RecommendationAutocompleteCommand struct {
	Meta
}

func (r *RecommendationAutocompleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := r.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Recommendations, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Recommendations]
	})
}

// RecommendationDismissCommand implements cli.Command.
type RecommendationDismissCommand struct {
	RecommendationAutocompleteCommand
}

// Help satisfies the cli.Command Help function.
func (r *RecommendationDismissCommand) Help() string {
	helpText := `
Usage: nomad recommendation dismiss [options] <recommendation_ids>

  Dismiss one or more Nomad recommendations.

  When ACLs are enabled, this command requires a token with the 'submit-job',
  'read-job', and 'submit-recommendation' capabilities for the
  recommendation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (r *RecommendationDismissCommand) Synopsis() string {
	return "Dismiss one or more Nomad recommendations"
}

func (r *RecommendationDismissCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(r.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

// Name returns the name of this command.
func (r *RecommendationDismissCommand) Name() string { return "recommendation dismiss" }

// Run satisfies the cli.Command Run function.
func (r *RecommendationDismissCommand) Run(args []string) int {

	flags := r.Meta.FlagSet(r.Name(), FlagSetClient)
	flags.Usage = func() { r.Ui.Output(r.Help()) }
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

	// Create a list of recommendations to dismiss.
	ids := make([]string, len(args))
	copy(ids, args)

	_, err = client.Recommendations().Delete(ids, nil)
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error dismissing recommendations: %v", err))
		return 1
	}

	verb := "recommendation"
	if len(ids) > 1 {
		verb += "s"
	}
	r.Ui.Output(fmt.Sprintf("Successfully dismissed %s", verb))
	return 0
}
