package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobAllocsCommand struct {
	Meta
}

func (c *JobAllocsCommand) Help() string {
	helpText := `
Usage: nomad job allocs [options] <job>

  Display allocations for a particular job.

  When ACLs are enabled, this command requires a token with the 'read-job' and
  'list-jobs' capabilities for the job's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Allocs Options:

  -all
    Display all allocations matching the job ID, even those from an older
    instance of the job.

  -json
    Output the allocations in a JSON format.

  -t
    Format and display allocations using a Go template.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobAllocsCommand) Synopsis() string {
	return "List allocations for a job"
}

func (c *JobAllocsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
			"-verbose": complete.PredictNothing,
			"-all":     complete.PredictNothing,
		})
}

func (c *JobAllocsCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobAllocsCommand) Name() string { return "job allocs" }

func (c *JobAllocsCommand) Run(args []string) int {
	var json, verbose, all bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&all, "all", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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
	q := &api.QueryOptions{Namespace: jobs[0].JobSummary.Namespace}

	allocs, _, err := client.Jobs().Allocations(jobID, all, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving allocations: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, allocs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	c.Ui.Output(formatAllocListStubs(allocs, verbose, length))
	return 0
}
