package command

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
)

type RunCommand struct {
	Meta
}

func (c *RunCommand) Help() string {
	helpText := `
Usage: nomad run [options] <file>

  Starts running a new job using the definition located at <file>.
  This is the main command used to invoke new work in Nomad.

General Options:

  ` + generalOptionsUsage() + `

Run Options:

  -monitor
    On successful job completion, immediately begin monitoring the
    evaluation created by the job registration. This mode will
    enter an interactive session where status is printed to the
    screen, similar to the "tail" UNIX command.
`
	return strings.TrimSpace(helpText)
}

func (c *RunCommand) Synopsis() string {
	return "Run a new job"
}

func (c *RunCommand) Run(args []string) int {
	var monitor bool

	flags := c.Meta.FlagSet("run", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&monitor, "monitor", false, "")

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

	// Convert it to something we can use
	apiJob, err := convertJob(job)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error converting job: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Submit the job
	evalID, _, err := client.Jobs().Register(apiJob, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error submitting job: %s", err))
		return 1
	}

	// Check if we should enter monitor mode
	if monitor {
		mon := newMonitor(c.Ui, client)
		return mon.monitor(evalID)
	}

	// By default just print some info and return
	c.Ui.Output("JobID  = " + job.ID)
	c.Ui.Output("EvalID = " + evalID)
	return 0
}

// convertJob is used to take a *structs.Job and convert it to an *api.Job.
// This function is just a hammer and probably needs to be revisited.
func convertJob(in *structs.Job) (*api.Job, error) {
	var apiJob *api.Job
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(in); err != nil {
		return nil, err
	}
	if err := gob.NewDecoder(buf).Decode(&apiJob); err != nil {
		return nil, err
	}
	return apiJob, nil
}
