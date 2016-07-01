package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/jobspec"
)

type ValidateCommand struct {
	Meta
}

func (c *ValidateCommand) Help() string {
	helpText := `
Usage: nomad validate [options] <file>

  Checks if a given HCL job file has a valid specification. This can be used to
  check for any syntax errors or validation problems with a job.

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
	file := args[0]

	// Parse the job file
	job, err := jobspec.ParseFile(file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing job file %s: %s", file, err))
		return 1
	}

	// Initialize any fields that need to be.
	job.InitFields()

	// Check that the job is valid
	if err := job.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}

	// Done!
	c.Ui.Output("Job validation successful")
	return 0
}
