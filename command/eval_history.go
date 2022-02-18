package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type EvalHistoryCommand struct {
	Meta
}

func (c *EvalHistoryCommand) Help() string {
	helpText := `
Usage: nomad eval history [options]

  History is used to show the set of evaluations related to an evaluation's history.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval History Options:

  -verbose
    Show full information.

  -filter
    Specifies an expression used to filter query results.

  -json
    Output the evaluation in its JSON format.

  -t
    Format and display evaluation using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *EvalHistoryCommand) Synopsis() string {
	return "History is used to show the set of evaluations related to an evaluation's history"
}

func (c *EvalHistoryCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":       complete.PredictNothing,
			"-t":          complete.PredictAnything,
			"-verbose":    complete.PredictNothing,
			"-filter":     complete.PredictAnything,
		})
}

func (c *EvalHistoryCommand) AutocompleteArgs() complete.Predictor {
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

func (c *EvalHistoryCommand) Name() string { return "eval history" }

func (c *EvalHistoryCommand) Run(args []string) int {
	var monitor, verbose, json bool
	var perPage int
	var tmpl, pageToken, filter string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&monitor, "monitor", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.StringVar(&filter, "filter", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes exactly one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	evalID := args[0]

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

	evals, qm, err := client.Evaluations().History(evalID, opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying evaluation history: %v", err))
		return 1
	}

	// If args not specified but output format is specified, format
	// and output the evaluations data History
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

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	out := make([]string, len(evals)+1)
	i := 0
	out[i] = "ID|Priority|Triggered By|Job ID|Status|Placement Failures"
	for id, eval := range evals {
		failures, _ := evalFailureStatus(eval)
		out[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s|%s",
			limit(id, length),
			eval.Priority,
			eval.TriggeredBy,
			eval.JobID,
			eval.Status,
			failures,
		)
	}
	c.Ui.Output(formatKV(out))

	if qm.NextToken != "" {
		c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
	}

	return 0
}

// TODO: removed for now
// argsWithoutPageToken strips out of the -page-token argument and
// returns the joined string
//func argsWithoutPageToken(osArgs []string) string {
//	args := []string{}
//	i := 0
//	for {
//		if i >= len(osArgs) {
//			break
//		}
//		arg := osArgs[i]
//
//		if strings.HasPrefix(arg, "-page-token") {
//			if strings.Contains(arg, "=") {
//				i += 1
//			} else {
//				i += 2
//			}
//			continue
//		}
//
//		args = append(args, arg)
//		i++
//	}
//	return strings.Join(args, " ")
//}
