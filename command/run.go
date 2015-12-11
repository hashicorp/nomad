package command

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

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

  Starts running a new job or updates an existing job using
  the specification located at <file>. This is the main command
  used to interact with Nomad.

  Upon successful job submission, this command will immediately
  enter an interactive monitor. This is useful to watch Nomad's
  internals make scheduling decisions and place the submitted work
  onto nodes. The monitor will end once job placement is done. It
  is safe to exit the monitor early using ctrl+c.

  On successful job submission and scheduling, exit code 0 will be
  returned. If there are job placement issues encountered
  (unsatisfiable constraints, resource exhaustion, etc), then the
  exit code will be 2. Any other errors, including client connection
  issues or internal errors, are indicated by exit code 1.

General Options:

  ` + generalOptionsUsage() + `

Run Options:

  -detach
    Return immediately instead of entering monitor mode. After job
    submission, the evaluation ID will be printed to the screen.
    You can use this ID to start a monitor using the eval-monitor
    command later if needed.
`
	return strings.TrimSpace(helpText)
}

func (c *RunCommand) Synopsis() string {
	return "Run a new job or update an existing job"
}

func (c *RunCommand) Run(args []string) int {
	var detach bool

	flags := c.Meta.FlagSet("run", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")

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

	// Check that the job is valid
	if err := job.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}

	// Check if the job is periodic.
	periodic := job.IsPeriodic()

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
	if detach || periodic {
		c.Ui.Output("Job registration successful")
		if periodic {
			c.Ui.Output(fmt.Sprintf("Approximate next launch time: %v", job.Periodic.Next(time.Now())))
		} else {
			c.Ui.Output("Evaluation ID: " + evalID)
		}

		return 0
	}

	// Detach was not specified, so start monitoring
	mon := newMonitor(c.Ui, client)
	return mon.monitor(evalID)

}

// convertJob is used to take a *structs.Job and convert it to an *api.Job.
// This function is just a hammer and probably needs to be revisited.
func convertJob(in *structs.Job) (*api.Job, error) {
	gob.Register([]map[string]interface{}{})
	gob.Register([]interface{}{})
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
