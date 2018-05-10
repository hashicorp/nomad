package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobEvalCommand struct {
	Meta
	forceRescheduling bool
}

func (c *JobEvalCommand) Help() string {
	helpText := `
Usage: nomad job eval [options] <job_id>

  Force an evaluation of the provided job ID. Forcing an evaluation will trigger the scheduler
  to re-evaluate the job. The force flags allow operators to force the scheduler to create
  new allocations under certain scenarios.

General Options:

  ` + generalOptionsUsage() + `

Eval Options:

  -force-reschedule
    Force reschedule failed allocations even if they are not currently
    eligible for rescheduling.
`
	return strings.TrimSpace(helpText)
}

func (c *JobEvalCommand) Synopsis() string {
	return "Force an evaluation for the job using its job ID"
}

func (c *JobEvalCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-force-reschedule": complete.PredictNothing,
		})
}

func (c *JobEvalCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Jobs]
	})
}

func (c *JobEvalCommand) Name() string { return "job eval" }

func (c *JobEvalCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.forceRescheduling, "force-reschedule", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Call eval endpoint
	jobID := args[0]

	opts := api.EvalOptions{
		ForceReschedule: c.forceRescheduling,
	}
	evalId, _, err := client.Jobs().ForceEvaluate(jobID, opts, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error evaluating job: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Created eval ID: %q ", evalId))
	return 0
}
