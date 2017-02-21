package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/colorstring"
)

type ValidateCommand struct {
	Meta
	JobGetter
	color *colorstring.Colorize
}

func (c *ValidateCommand) Help() string {
	helpText := `
Usage: nomad validate [options] <path>

  Checks if a given HCL job file has a valid specification. This can be used to
  check for any syntax errors or validation problems with a job.

  If the supplied path is "-", the jobfile is read from stdin. Otherwise
  it is read from the file at the supplied path or downloaded and
  read from URL specified.
`
	return strings.TrimSpace(helpText)
}

func (c *ValidateCommand) Synopsis() string {
	return "Checks if a given job specification is valid"
}

func (c *ValidateCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("validate", FlagSetNone)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get Job struct from Jobfile
	job, err := c.JobGetter.ApiJob(args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting job struct: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 255
	}

	// Force the region to be that of the job.
	if r := job.Region; r != nil {
		client.SetRegion(*r)
	}

	// Check that the job is valid
	jr, _, err := client.Jobs().Validate(job, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}
	if jr != nil && !jr.DriverConfigValidated {
		c.Ui.Output(c.Colorize().Color("[bold][orange]Driver configuration not validated.[reset]"))
	}

	if jr != nil && len(jr.ValidationErrors) > 0 {
		c.Ui.Output("Job Validation errors:")
		for _, err := range jr.ValidationErrors {
			c.Ui.Output(err)
		}
		return 1
	}

	// Done!
	c.Ui.Output("Job validation successful")
	return 0
}
