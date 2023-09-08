// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure RecommendationListCommand satisfies the cli.Command interface.
var _ cli.Command = &RecommendationListCommand{}

// RecommendationListCommand implements cli.Command.
type RecommendationListCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (r *RecommendationListCommand) Help() string {
	helpText := `
Usage: nomad recommendation list [options]

  List is used to list the available recommendations.

  When ACLs are enabled, this command requires a token with the 'submit-job',
  'read-job', and 'submit-recommendation' capabilities for the namespace being
  queried.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Recommendation List Options:

  -job
    Specifies the job ID to filter the recommendations list by.

  -group
    Specifies the task group name to filter within a job. If specified, the -job
    flag must also be specified.

  -task
    Specifies the task name to filter within a job and task group. If specified,
    the -job and -group flags must also be specified.

  -json
    Output the recommendations in JSON format.

  -t
    Format and display the recommendations using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (r *RecommendationListCommand) Synopsis() string {
	return "Display all Nomad recommendations"
}

func (r *RecommendationListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(r.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-job":   complete.PredictNothing,
			"-group": complete.PredictNothing,
			"-task":  complete.PredictNothing,
			"-json":  complete.PredictNothing,
			"-t":     complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (r *RecommendationListCommand) Name() string { return "recommendation list" }

// Run satisfies the cli.Command Run function.
func (r *RecommendationListCommand) Run(args []string) int {
	var json bool
	var tmpl, job, group, task string

	flags := r.Meta.FlagSet(r.Name(), FlagSetClient)
	flags.Usage = func() { r.Ui.Output(r.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.StringVar(&job, "job", "", "")
	flags.StringVar(&group, "group", "", "")
	flags.StringVar(&task, "task", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		r.Ui.Error("This command takes no arguments")
		r.Ui.Error(commandErrorText(r))
	}

	// Get the HTTP client.
	client, err := r.Meta.Client()
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Validate the input flags. This is done by the HTTP API anyway, but there
	// is no harm doing it here to avoid calls that we know wont succeed.
	if group != "" && job == "" {
		r.Ui.Error("Job flag must be supplied when using group flag")
		return 1

	}
	if task != "" && group == "" {
		r.Ui.Error("Group flag must be supplied when using task flag")
		return 1
	}

	// Setup the query params.
	q := &api.QueryOptions{
		Params: map[string]string{},
	}
	if job != "" {
		q.Params["job"] = job
	}
	if group != "" {
		q.Params["group"] = group
	}
	if task != "" {
		q.Params["task"] = task
	}

	recommendations, _, err := client.Recommendations().List(q)
	if err != nil {
		r.Ui.Error(fmt.Sprintf("Error listing recommendations: %s", err))
		return 1
	}

	if len(recommendations) == 0 {
		r.Ui.Output("No recommendations found")
		return 0
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, recommendations)
		if err != nil {
			r.Ui.Error(err.Error())
			return 1
		}
		r.Ui.Output(out)
		return 0
	}

	// Create the output table header.
	output := []string{"ID|"}

	// If the operator is using the namespace wildcard option, add this header.
	if r.Meta.namespace == "*" {
		output[0] += "Namespace|"
	}
	output[0] += "Job|Group|Task|Resource|Value"

	// Sort the list of recommendations based on their job, group and task.
	sortedRecs := recommendationList{r: recommendations}
	sort.Sort(sortedRecs)

	// Iterate the recommendations and add to the output.
	for i, rec := range sortedRecs.r {

		output = append(output, rec.ID)

		if r.Meta.namespace == "*" {
			output[i+1] += fmt.Sprintf("|%s", rec.Namespace)
		}
		output[i+1] += fmt.Sprintf("|%s|%s|%s|%s|%v", rec.JobID, rec.Group, rec.Task, rec.Resource, rec.Value)
	}

	// Output.
	r.Ui.Output(formatList(output))
	return 0
}

// recommendationList is a wrapper around []*api.Recommendation that lets us
// sort the recommendations alphabetically based on their job, group and task.
type recommendationList struct {
	r []*api.Recommendation
}

// Len satisfies the Len function of the sort.Interface interface.
func (r recommendationList) Len() int { return len(r.r) }

// Swap satisfies the Swap function of the sort.Interface interface.
func (r recommendationList) Swap(i, j int) {
	r.r[i], r.r[j] = r.r[j], r.r[i]
}

// Less satisfies the Less function of the sort.Interface interface.
func (r recommendationList) Less(i, j int) bool {
	recI := r.stringFromResource(i)
	recJ := r.stringFromResource(j)
	stringList := []string{recI, recJ}
	sort.Strings(stringList)
	return stringList[0] == recI
}

func (r recommendationList) stringFromResource(i int) string {
	return strings.Join([]string{r.r[i].Namespace, r.r[i].JobID, r.r[i].Group, r.r[i].Task, r.r[i].Resource}, ":")
}
