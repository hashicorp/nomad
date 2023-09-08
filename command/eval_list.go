// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type EvalListCommand struct {
	Meta
}

func (c *EvalListCommand) Help() string {
	helpText := `
Usage: nomad eval list [options]

  List is used to list the set of evaluations processed by Nomad.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval List Options:

  -verbose
    Show full information.

  -per-page
    How many results to show per page.

  -page-token
    Where to start pagination.

  -filter
    Specifies an expression used to filter query results.

  -job
    Only show evaluations for this job ID.

  -status
    Only show evaluations with this status.

  -json
    Output the evaluation in its JSON format.

  -t
    Format and display evaluation using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *EvalListCommand) Synopsis() string {
	return "List the set of evaluations processed by Nomad"
}

func (c *EvalListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":       complete.PredictNothing,
			"-t":          complete.PredictAnything,
			"-verbose":    complete.PredictNothing,
			"-filter":     complete.PredictAnything,
			"-job":        complete.PredictAnything,
			"-status":     complete.PredictAnything,
			"-per-page":   complete.PredictAnything,
			"-page-token": complete.PredictAnything,
		})
}

func (c *EvalListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Evals, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Evals]
	})
}

func (c *EvalListCommand) Name() string { return "eval list" }

func (c *EvalListCommand) Run(args []string) int {
	var monitor, verbose, json bool
	var perPage int
	var tmpl, pageToken, filter, filterJobID, filterStatus string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&monitor, "monitor", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.IntVar(&perPage, "per-page", 0, "")
	flags.StringVar(&pageToken, "page-token", "", "")
	flags.StringVar(&filter, "filter", "", "")
	flags.StringVar(&filterJobID, "job", "", "")
	flags.StringVar(&filterStatus, "status", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	opts := &api.QueryOptions{
		Filter:    filter,
		PerPage:   int32(perPage),
		NextToken: pageToken,
		Params:    map[string]string{},
	}
	if filterJobID != "" {
		opts.Params["job"] = filterJobID
	}
	if filterStatus != "" {
		opts.Params["status"] = filterStatus
	}

	evals, qm, err := client.Evaluations().List(opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying evaluations: %v", err))
		return 1
	}

	// If args not specified but output format is specified, format
	// and output the evaluations data list
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, evals)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	if len(evals) == 0 {
		c.Ui.Output("No evals found")
		return 0
	}

	c.Ui.Output(formatEvalList(evals, verbose))

	if qm.NextToken != "" {
		c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
	}

	return 0
}

// argsWithoutPageToken strips out of the -page-token argument and
// returns the joined string
func argsWithoutPageToken(osArgs []string) string {
	args := []string{}
	i := 0
	for {
		if i >= len(osArgs) {
			break
		}
		arg := osArgs[i]

		if strings.HasPrefix(arg, "-page-token") {
			if strings.Contains(arg, "=") {
				i += 1
			} else {
				i += 2
			}
			continue
		}

		args = append(args, arg)
		i++
	}
	return strings.Join(args, " ")
}

func formatEvalList(evals []*api.Evaluation, verbose bool) string {
	// Truncate IDs unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	out := make([]string, len(evals)+1)
	out[0] = "ID|Priority|Triggered By|Job ID|Namespace|Node ID|Status|Placement Failures"
	for i, eval := range evals {
		failures, _ := evalFailureStatus(eval)
		out[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%s",
			limit(eval.ID, length),
			eval.Priority,
			eval.TriggeredBy,
			eval.JobID,
			eval.Namespace,
			limit(eval.NodeID, length),
			eval.Status,
			failures,
		)
	}

	return formatList(out)
}
