package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobRestartCommand struct {
	Meta
	BatchSize string
	BatchWait int
}

func (c *JobRestartCommand) Help() string {
	helpText := `
Usage: nomad job restart [options] <job>

  Restart allocations for a particular job in batches.

  When ACLs are enabled, this command requires a token with the
  'alloc-lifecycle', 'read-job', and 'list-jobs' capabilities for the
  allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Allocs Options:

  -batch-size
    Number of allocations to restart at once.

  -batch-wait
    Wait time in seconds between each batch restart.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRestartCommand) Synopsis() string {
	return "Restart all allocations for a job"
}

func (c *JobRestartCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-batch-size": complete.PredictNothing,
			"-batch-wait": complete.PredictAnything,
		})
}

func (c *JobRestartCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobRestartCommand) Name() string { return "restart job and all it's allocations" }

func (c *JobRestartCommand) Run(args []string) int {
	var batchSize string
	var batchWait int

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&batchSize, "batch-size", "5", "")
	flags.IntVar(&batchWait, "batch-wait", 10, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one job
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

	jobID := strings.TrimSpace(args[0])

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing jobs: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 {
		if (jobID != jobs[0].ID) || (c.allNamespaces() && jobs[0].ID == jobs[1].ID) {
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces())))
			return 1
		}
	}

	jobID = jobs[0].ID
	q := &api.WriteOptions{Namespace: jobs[0].JobSummary.Namespace}

	client.Jobs().Restart(jobID, q, batchSize, batchWait)
	return 0
}
