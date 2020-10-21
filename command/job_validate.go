package command

import (
	"fmt"
	"strings"

	cflags "github.com/hashicorp/consul/command/flags"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
)

type JobValidateCommand struct {
	Meta
	JobGetter
}

func (c *JobValidateCommand) Help() string {
	helpText := `
Usage: nomad job validate [options] <path>
Alias: nomad validate

  Checks if a given HCL job file has a valid specification. This can be used to
  check for any syntax errors or validation problems with a job.

  If the supplied path is "-", the jobfile is read from stdin. Otherwise
  it is read from the file at the supplied path or downloaded and
  read from URL specified.

Validate Options:

  -hcl1
    Parses the job file as HCLv1.
`
	return strings.TrimSpace(helpText)
}

func (c *JobValidateCommand) Synopsis() string {
	return "Checks if a given job specification is valid"
}

func (c *JobValidateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-hcl1": complete.PredictNothing,
		"-var":  complete.PredictAnything,
	}
}

func (c *JobValidateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(complete.PredictFiles("*.nomad"), complete.PredictFiles("*.hcl"))
}

func (c *JobValidateCommand) Name() string { return "job validate" }

func (c *JobValidateCommand) Run(args []string) int {
	var varArgs cflags.AppendSliceValue

	flags := c.Meta.FlagSet(c.Name(), FlagSetNone)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.JobGetter.hcl1, "hcl1", false, "")
	flags.Var(&varArgs, "var", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get Job struct from Jobfile
	job, err := c.JobGetter.ApiJobWithArgs(args[0], parseVars(varArgs))
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
func (c *JobValidateCommand) validateLocal(aj *api.Job) (*api.JobValidateResponse, error) {
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
