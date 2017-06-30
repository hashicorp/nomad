package command

import (
	"fmt"
	"strings"
)

type JobHistoryCommand struct {
	Meta
}

func (c *JobHistoryCommand) Help() string {
	helpText := `
Usage: nomad job history [options] <job>

History is used to display the known versions of a particular job. The command
can display the diff between job versions and can be useful for understanding
the changes that occured to the job as well as deciding job versions to revert
to.

General Options:

  ` + generalOptionsUsage() + `

History Options:

  -p
    Display the difference between each job and its predecessor.
    
  -full
    Display the full job definition for each version.

  -version <job version>
    Display only the history for the given job version.
`
	return strings.TrimSpace(helpText)
}

func (c *JobHistoryCommand) Synopsis() string {
	return "Display all tracked versions of a job"
}

func (c *JobHistoryCommand) Run(args []string) int {
	var diff, full bool
	var version uint64

	flags := c.Meta.FlagSet("job history", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&diff, "p", false, "")
	flags.BoolVar(&full, "full", false, "")
	flags.Uint64Var(&version, "version", 0, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if l := len(args); l < 1 || l > 2 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	jobID := args[0]
	versions, _, err := client.Jobs().Versions(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
		return 1
	}

	c.Ui.Output(jobID)
	c.Ui.Output(fmt.Sprintf("%d", len(versions)))
	return 0
}
