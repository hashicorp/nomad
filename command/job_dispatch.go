package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	flaghelper "github.com/hashicorp/nomad/helper/flag-helpers"
)

type JobDispatchCommand struct {
	Meta
}

func (c *JobDispatchCommand) Help() string {
	helpText := `
Usage: nomad job dispatch [options] <dispatch-template>


General Options:

  ` + generalOptionsUsage() + `

Dispatch Options:

`
	return strings.TrimSpace(helpText)
}

func (c *JobDispatchCommand) Synopsis() string {
	return "Dispatch an instance of a dispatch template"
}

func (c *JobDispatchCommand) Run(args []string) int {
	var detach, verbose bool
	var meta []string
	var inputFile string

	flags := c.Meta.FlagSet("job dispatch", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&inputFile, "input-file", "", "")
	flags.Var((*flaghelper.StringFlag)(&meta), "meta", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	//length := shortId
	//if verbose {
	//length = fullId
	//}

	// Check that we got exactly one node
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	templateJob := args[0]
	var inputData []byte
	var readErr error

	// If the input data is specified try to read from the file
	if inputFile != "" {
		inputData, readErr = ioutil.ReadFile(inputFile)
	} else {
		// Read from stdin
		inputData, readErr = ioutil.ReadAll(os.Stdin)
	}
	if readErr != nil {
		c.Ui.Error(fmt.Sprintf("Error reading input data: %v", readErr))
		return 1
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
	resp, _, err := client.Jobs().Dispatch(templateJob, metaMap, inputData, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to dispatch job: %s", err))
		return 1
	}

	basic := []string{
		fmt.Sprintf("Dispatched Job ID|%s", resp.DispatchedJobID),
		fmt.Sprintf("Evaluation ID|%s", resp.EvalID),
	}
	c.Ui.Output(formatKV(basic))
	return 0
}
