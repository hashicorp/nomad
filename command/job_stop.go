package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobStopCommand struct {
	Meta
}

func (c *JobStopCommand) Help() string {
	helpText := `
Usage: nomad job stop [options] <job>
Alias: nomad stop

  Stop an existing job. This command is used to signal allocations
  to shut down for the given job ID. Upon successful deregistration,
  an interactive monitor session will start to display log lines as
  the job unwinds its allocations and completes shutting down. It
  is safe to exit the monitor early using ctrl+c.

General Options:

  ` + generalOptionsUsage() + `

Stop Options:

  -detach
    Return immediately instead of entering monitor mode. After the
    deregister command is submitted, a new evaluation ID is printed to the
    screen, which can be used to examine the evaluation using the eval-status
    command.

  -purge
    Purge is used to stop the job and purge it from the system. If not set, the
    job will still be queryable and will be purged by the garbage collector.

  -global
    Stop a multi-region job in all its regions. By default job stop will stop
    only a single region at a time. Ignored for single-region jobs.

  -yes
    Automatic yes to prompts.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobStopCommand) Synopsis() string {
	return "Stop a running job"
}

func (c *JobStopCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-purge":   complete.PredictNothing,
			"-global":  complete.PredictNothing,
			"-yes":     complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobStopCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobStopCommand) Name() string { return "job stop" }

func (c *JobStopCommand) Run(args []string) int {
	var detach, purge, verbose, global, autoYes bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&global, "global", false, "")
	flags.BoolVar(&autoYes, "yes", false, "")
	flags.BoolVar(&purge, "purge", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got exactly one job
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	jobID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 && (c.allNamespaces() || strings.TrimSpace(jobID) != jobs[0].ID) {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces())))
		return 1
	}
	// Prefix lookup matched a single job
	q := &api.QueryOptions{Namespace: jobs[0].JobSummary.Namespace}
	job, _, err := client.Jobs().Info(jobs[0].ID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	getConfirmation := func(question string) (int, bool) {
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1, false
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Cancelling job stop")
			return 0, false
		} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
			// Non exact match yes
			c.Ui.Output("For confirmation, an exact ‘y’ is required.")
			return 0, false
		} else if answer != "y" {
			c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
			return 1, false
		}
		return 0, true
	}

	// Confirm the stop if the job was a prefix match
	if jobID != *job.ID && !autoYes {
		question := fmt.Sprintf("Are you sure you want to stop job %q? [y/N]", *job.ID)
		code, confirmed := getConfirmation(question)
		if !confirmed {
			return code
		}
	}

	// Confirm we want to stop only a single region of a multiregion job
	if job.IsMultiregion() && !global {
		question := fmt.Sprintf(
			"Are you sure you want to stop multi-region job %q in a single region? [y/N]", *job.ID)
		code, confirmed := getConfirmation(question)
		if !confirmed {
			return code
		}
	}

	// Invoke the stop
	opts := &api.DeregisterOptions{Purge: purge, Global: global}
	wq := &api.WriteOptions{Namespace: jobs[0].JobSummary.Namespace}
	evalID, _, err := client.Jobs().DeregisterOpts(*job.ID, opts, wq)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	// If we are stopping a periodic job there won't be an evalID.
	if evalID == "" {
		return 0
	}

	if detach {
		c.Ui.Output(evalID)
		return 0
	}

	// Start monitoring the stop eval
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(evalID, false)
}
