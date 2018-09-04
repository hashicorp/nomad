package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobScaleCommand struct {
	Meta
}

func (c *JobScaleCommand) Help() string {
	helpText := `
	Usage: nomad job scale [options] <job> <task_group:scale>...
Alias: nomad scale

  Scale an existing job. This command is used to change the count of
  existing task groups within a job.

General Options:

  ` + generalOptionsUsage() + `
`
	return strings.TrimSpace(helpText)
}

func (c *JobScaleCommand) Synopsis() string {
	return "Scale a running job"
}

func (c *JobScaleCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *JobScaleCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobScaleCommand) Name() string { return "job scale" }

func (c *JobScaleCommand) Run(args []string) int {

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one job
	args = flags.Args()
	if len(args) < 1 {
		c.Ui.Error("This command takes one or more argument: <job> <tg:scale>...")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	jobID := args[0]

	// Validate ops
	scaleOps := map[string]string{}
	for _, op := range args[1:] {
		parts := strings.Split(op, ":")
		if len(parts) != 2 {
			c.Ui.Error(fmt.Sprintf("Bad scale format: <task group>:<operation>"))
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		strVal := parts[1]
		if strings.HasPrefix(parts[1], "-") || strings.HasPrefix(parts[1], "+") {
			strVal = parts[1][1:]
		}
		_, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not parse operation for task group '%s': %v",
				parts[0], err))
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		scaleOps[parts[0]] = parts[1]
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error scaling job: %s", err))
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
	job, _, err := client.Jobs().Info(jobs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering job: %s", err))
		return 1
	}

	// Invoke the scale
	resp, _, err := client.Jobs().Scale(*job.ID, scaleOps, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error scaling job: %s", err))
		return 1
	}

	fmt.Println(resp)
	return 0
}
