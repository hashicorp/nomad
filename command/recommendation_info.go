// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure RecommendationInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &RecommendationInfoCommand{}

// RecommendationInfoCommand implements cli.Command.
type RecommendationInfoCommand struct {
	RecommendationAutocompleteCommand
}

// Help satisfies the cli.Command Help function.
func (r *RecommendationInfoCommand) Help() string {
	helpText := `
Usage: nomad recommendation info [options] <recommendation_id>

  Info is used to read the specified recommendation.

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the recommendation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Recommendation Info Options:

  -json
    Output the recommendation in its JSON format.

  -t
    Format and display the recommendation using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (r *RecommendationInfoCommand) Synopsis() string {
	return "Display an individual Nomad recommendation"
}

func (r *RecommendationInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(r.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (r *RecommendationInfoCommand) Name() string { return "recommendation info" }

// Run satisfies the cli.Command Run function.
func (r *RecommendationInfoCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := r.Meta.FlagSet(r.Name(), FlagSetClient)
	flags.Usage = func() { r.Ui.Output(r.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) != 1 {
		r.Ui.Error("This command takes one argument: <recommendation_id>")
		r.Ui.Error(commandErrorText(r))
		return 1
	}

	// Get the recommendation ID.
	recID := args[0]

	// Get the HTTP client.
	client, err := r.Meta.Client()
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	rec, _, err := client.Recommendations().Info(recID, nil)
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error reading recommendation: %s", err))
		return 1
	}

	// If the user has specified to output the recommendation as JSON or using
	// a template then perform this action for the entire object and exit the
	// command.
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, rec)
		if err != nil {
			r.Ui.Error(err.Error())
			return 1
		}
		r.Ui.Output(out)
		return 0
	}

	info := []string{
		fmt.Sprintf("ID|%s", rec.ID),
		fmt.Sprintf("Namespace|%s", rec.Namespace),
		fmt.Sprintf("Job ID|%s", rec.JobID),
		fmt.Sprintf("Task Group|%s", rec.Group),
		fmt.Sprintf("Task|%s", rec.Task),
		fmt.Sprintf("Resource|%s", rec.Resource),
		fmt.Sprintf("Value|%v", rec.Value),
		fmt.Sprintf("Current|%v", rec.Current),
	}
	r.Ui.Output(formatKV(info))

	// If we have stats, format and output these.
	if len(rec.Stats) > 0 {

		// Sort the stats keys into an alphabetically ordered list to provide
		// consistent outputs.
		keys := []string{}

		for k := range rec.Stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// We will only need two rows; key:value.
		output := make([]string, 2)

		for _, stat := range keys {
			output[0] += fmt.Sprintf("%s|", stat)
			output[1] += fmt.Sprintf("%.2f|", rec.Stats[stat])
		}

		// Trim any trailing pipes so we can use the formatList function thus
		// providing a nice clean output.
		output[0] = strings.TrimRight(output[0], "|")
		output[1] = strings.TrimRight(output[1], "|")

		r.Ui.Output(r.Colorize().Color("\n[bold]Stats[reset]"))
		r.Ui.Output(formatList(output))
	}

	// If we have meta, format and output the entries.
	if len(rec.Meta) > 0 {

		// Sort the meta keys into an alphabetically ordered list to provide
		// consistent outputs.
		keys := []string{}

		for k := range rec.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		output := make([]string, len(rec.Meta))

		for i, key := range keys {
			output[i] = fmt.Sprintf("%s|%v", key, rec.Meta[key])
		}
		r.Ui.Output(r.Colorize().Color("\n[bold]Meta[reset]"))
		r.Ui.Output(formatKV(output))
	}

	return 0
}
