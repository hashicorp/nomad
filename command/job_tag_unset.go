// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type JobTagUnsetCommand struct {
	Meta
}

func (c *JobTagUnsetCommand) Help() string {
	helpText := `
Usage: nomad job tag unset [options] -name <tag> <job>

  Remove a tag from a job version. This command requires a job ID and a tag name.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Tag Unset Options:

  -name
    The name of the tag to remove from the job version.

`
	return strings.TrimSpace(helpText)
}

func (c *JobTagUnsetCommand) Synopsis() string {
	return "Remove a tag from a job version."
}

func (c *JobTagUnsetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *JobTagUnsetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *JobTagUnsetCommand) Name() string { return "job tag unset" }

func (c *JobTagUnsetCommand) Run(args []string) int {
	var name string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if len(flags.Args()) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var job = flags.Args()[0]

	if job == "" {
		c.Ui.Error(
			"A job name is required",
		)
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if name == "" {
		c.Ui.Error(
			"A version tag name is required",
		)
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(job)
	jobID, _, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	_, err = client.Jobs().UntagVersion(jobID, name, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error tagging job version: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Tag \"%s\" removed from job \"%s\"", name, job))

	return 0
}
