package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobRevertCommand struct {
	Meta
}

func (c *JobRevertCommand) Help() string {
	helpText := `
Usage: nomad job revert [options] <job> <version>

  Revert is used to revert a job to a prior version of the job. The available
  versions to revert to can be found using "nomad job history" command.

General Options:

  ` + generalOptionsUsage() + `

Revert Options:

  -detach
    Return immediately instead of entering monitor mode. After job revert,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -vault-token 
   The Vault token used to verify that the caller has access to the Vault 
   policies i the targeted version of the job.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRevertCommand) Synopsis() string {
	return "Revert to a prior version of the job"
}

func (c *JobRevertCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobRevertCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobRevertCommand) Name() string { return "job revert" }

func (c *JobRevertCommand) Run(args []string) int {
	var detach, verbose bool
	var vaultToken string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&vaultToken, "vault-token", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got two args
	args = flags.Args()
	if l := len(args); l != 2 {
		c.Ui.Error("This command takes two arguments: <job> <version>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Parse the Vault token
	if vaultToken == "" {
		// Check the environment variable
		vaultToken = os.Getenv("VAULT_TOKEN")
	}

	jobID := args[0]
	revertVersion, ok, err := parseVersion(args[1])
	if !ok {
		c.Ui.Error("The job version to revert to must be specified using the -job-version flag")
		return 1
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse job-version flag: %v", err))
		return 1
	}

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
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 1
	}

	// Prefix lookup matched a single job
	resp, _, err := client.Jobs().Revert(jobs[0].ID, revertVersion, nil, nil, vaultToken)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
		return 1
	}

	// Nothing to do
	evalCreated := resp.EvalID != ""
	if detach || !evalCreated {
		return 0
	}

	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(resp.EvalID, false)
}
