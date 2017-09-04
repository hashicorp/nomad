package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	flaghelper "github.com/hashicorp/nomad/helper/flag-helpers"
	"github.com/posener/complete"
)

type JobDispatchCommand struct {
	Meta
}

func (c *JobDispatchCommand) Help() string {
	helpText := `
Usage: nomad job dispatch [options] <parameterized job> [input source]

Dispatch creates an instance of a parameterized job. A data payload to the
dispatched instance can be provided via stdin by using "-" or by specifying a
path to a file. Metadata can be supplied by using the meta flag one or more
times. 

Upon successful creation, the dispatched job ID will be printed and the
triggered evaluation will be monitored. This can be disabled by supplying the
detach flag.

General Options:

  ` + generalOptionsUsage() + `

Dispatch Options:

  -meta <key>=<value>
    Meta takes a key/value pair separated by "=". The metadata key will be
    merged into the job's metadata. The job may define a default value for the
    key which is overridden when dispatching. The flag can be provided more than
    once to inject multiple metadata key/value pairs. Arbitrary keys are not
    allowed. The parameterized job must allow the key to be merged.
    
  -detach
    Return immediately instead of entering monitor mode. After job dispatch,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobDispatchCommand) Synopsis() string {
	return "Dispatch an instance of a parameterized job"
}

func (c *JobDispatchCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-meta":    complete.PredictAnything,
			"-detach":  complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}

func (c *JobDispatchCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobDispatchCommand) Run(args []string) int {
	var detach, verbose bool
	var meta []string

	flags := c.Meta.FlagSet("job dispatch", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.Var((*flaghelper.StringFlag)(&meta), "meta", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got exactly one node
	args = flags.Args()
	if l := len(args); l < 1 || l > 2 {
		c.Ui.Error(c.Help())
		return 1
	}

	job := args[0]
	var payload []byte
	var readErr error

	// Read the input
	if len(args) == 2 {
		switch args[1] {
		case "-":
			payload, readErr = ioutil.ReadAll(os.Stdin)
		default:
			payload, readErr = ioutil.ReadFile(args[1])
		}
		if readErr != nil {
			c.Ui.Error(fmt.Sprintf("Error reading input data: %v", readErr))
			return 1
		}
	}

	// Build the meta
	metaMap := make(map[string]string, len(meta))
	for _, m := range meta {
		split := strings.SplitN(m, "=", 2)
		if len(split) != 2 {
			c.Ui.Error(fmt.Sprintf("Error parsing meta value: %v", m))
			return 1
		}

		metaMap[split[0]] = split[1]
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Dispatch the job
	resp, _, err := client.Jobs().Dispatch(job, metaMap, payload, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to dispatch job: %s", err))
		return 1
	}

	// See if an evaluation was created. If the job is periodic there will be no
	// eval.
	evalCreated := resp.EvalID != ""

	basic := []string{
		fmt.Sprintf("Dispatched Job ID|%s", resp.DispatchedJobID),
	}
	if evalCreated {
		basic = append(basic, fmt.Sprintf("Evaluation ID|%s", limit(resp.EvalID, length)))
	}
	c.Ui.Output(formatKV(basic))

	// Nothing to do
	if detach || !evalCreated {
		return 0
	}

	c.Ui.Output("")
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(resp.EvalID, false)
}
