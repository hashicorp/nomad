// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type JobTagApplyCommand struct {
	Meta
}

func (c *JobTagApplyCommand) Help() string {
	helpText := `
Usage: nomad job tag apply [options] <jobname>

  Save a job version to prevent it from being garbage-collected and allow it to
  be diffed and reverted by name.
  
  Example usage:
 
    nomad job tag apply -name "My Golden Version" \
		-description "The version we can roll back to if needed" <jobname>

    nomad job tag apply -version 3 -name "My Golden Version" <jobname>

  The first of the above will tag the latest version of the job, while the second
  will specifically tag version 3 of the job.

Tag Specific Options:

  -name <version-name>
    Specifies the name of the version to tag. This is a required field.

  -description <description>
    Specifies a description for the version. This is an optional field.

  -version <version>
    Specifies the version of the job to tag. If not provided, the latest version
    of the job will be tagged.


General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `
`
	return strings.TrimSpace(helpText)
}

func (c *JobTagApplyCommand) Synopsis() string {
	return "Save a job version to prevent it from being garbage-collected and allow it to be diffed and reverted by name."
}

func (c *JobTagApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":        complete.PredictAnything,
			"-description": complete.PredictAnything,
			"-version":     complete.PredictNothing,
		})
}

func (c *JobTagApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *JobTagApplyCommand) Name() string { return "job tag apply" }

func (c *JobTagApplyCommand) Run(args []string) int {
	var name, description, versionStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&description, "description", "", "")
	flags.StringVar(&versionStr, "version", "", "")

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
		c.Ui.Error("A job name is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if name == "" {
		c.Ui.Error("A version tag name is required")
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
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// If the version is not provided, get the "active" version of the job
	var versionInt uint64
	if versionStr == "" {
		q := &api.QueryOptions{
			Namespace: namespace,
		}
		latestVersion, _, err := client.Jobs().Info(job, q)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
			return 1
		}
		versionInt = *latestVersion.Version
	} else {
		var parseErr error
		versionInt, parseErr = strconv.ParseUint(versionStr, 10, 64)
		if parseErr != nil {
			c.Ui.Error(fmt.Sprintf("Error parsing version: %s", parseErr))
			return 1
		}
	}

	_, err = client.Jobs().TagVersion(jobID, versionInt, name, description, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error tagging job version: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Job version %d tagged with name \"%s\"", versionInt, name))

	return 0
}
