package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type RestartCommand struct {
	Meta
}

func (c *RestartCommand) Help() string {
	helpText := `
Usage: nomad restart [options] <job>

  Restart an existing job. This command is used to signal allocations to
  restart for the given job ID. Upon successful restart an interactive monitor
  session will start to display log lines as the job unwinds its allocations
  and completes restarts. It is safe to exit the monitor early using
  ctrl+c.

General Options:

  ` + generalOptionsUsage() + `

Restart Options:

  -detach
    Return immediately instead of entering monitor mode. After the
    restart command is submitted, a new evaluation ID is printed to the
    screen, which can be used to examine the evaluation using the eval-status
    command.

  -yes
    Automatic yes to prompts.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *RestartCommand) Synopsis() string {
	return "Restart a running job"
}

func (c *RestartCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-yes":     complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *RestartCommand) AutocompleteArgs() complete.Predictor {
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

func (c *RestartCommand) Run(args []string) int {
	var detach, verbose, autoYes bool

	flags := c.Meta.FlagSet("restart", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&autoYes, "yes", false, "")

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
		c.Ui.Error(c.Help())
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
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 1
	}
	// Prefix lookup matched a single job
	job, _, err := client.Jobs().Info(jobs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	// Confirm the restart if the job was a prefix match.
	if jobID != *job.ID && !autoYes {
		question := fmt.Sprintf("Are you sure you want to restart job %q? [y/N]", *job.ID)
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Cancelling job restart")
			return 0
		} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
			// Non exact match yes
			c.Ui.Output("For confirmation, an exact ‘y’ is required.")
			return 0
		} else if answer != "y" {
			c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
			return 1
		}
	}

	// Invoke the restart
	evalID, _, err := client.Jobs().Restart(*job.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error restart job: %s", err))
		return 1
	}

	// If we are restartping a periodic job there won't be an evalID.
	if evalID == "" {
		return 0
	}

	if detach {
		c.Ui.Output(evalID)
		return 0
	}

	// Start monitoring the restart eval
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(evalID, false)
}
