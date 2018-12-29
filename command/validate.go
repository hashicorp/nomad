package command

import (
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
)

type ValidateCommand struct {
	Meta
	JobGetter
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

func (c *ValidateCommand) AutocompleteFlags() complete.Flags {
	return nil
}

func (c *ValidateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(complete.PredictFiles("*.nomad"), complete.PredictFiles("*.hcl"))
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
		jr, err = c.validateLocal(job)
	}
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}

	if jr != nil && !jr.DriverConfigValidated {
		c.Ui.Output(
			c.Colorize().Color("[bold][yellow]Driver configuration not validated since connection to Nomad agent couldn't be established.[reset]\n"))
	}

	if jr != nil && jr.Error != "" {
		c.Ui.Error(
			c.Colorize().Color("[bold][red]Job validation errors:[reset]"))
		c.Ui.Error(jr.Error)
		return 1
	}

	// Print any warnings if there are any
	if jr.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(fmt.Sprintf("[bold][yellow]Job Warnings:\n%s[reset]\n", jr.Warnings)))
	}

	// Done!
	c.Ui.Output(
		c.Colorize().Color("[bold][green]Job validation successful[reset]"))
	return 0
}

// validateLocal validates without talking to a Nomad agent
func (c *ValidateCommand) validateLocal(aj *api.Job) (*api.JobValidateResponse, error) {
	var out api.JobValidateResponse

	job := agent.ApiJobToStructJob(aj)
	canonicalizeWarnings := job.Canonicalize()

	if vErr := job.Validate(); vErr != nil {
		if merr, ok := vErr.(*multierror.Error); ok {
			for _, err := range merr.Errors {
				out.ValidationErrors = append(out.ValidationErrors, err.Error())
			}
			out.Error = merr.Error()
		} else {
			out.ValidationErrors = append(out.ValidationErrors, vErr.Error())
			out.Error = vErr.Error()
		}
	}

	warnings := job.Warnings()
	out.Warnings = structs.MergeMultierrorWarnings(warnings, canonicalizeWarnings)
	return &out, nil
}
