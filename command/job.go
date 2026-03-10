// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobCommand struct {
	Meta
}

func (f *JobCommand) Help() string {
	helpText := `
Usage: nomad job <subcommand> [options] [args]

  This command groups subcommands for interacting with jobs.

  Run a new job or update an existing job:

      $ nomad job run <path>

  Plan the run of a job to determine what changes would occur:

      $ nomad job plan <path>

  Stop a running job:

      $ nomad job stop <name>

  Examine the status of a running job:

      $ nomad job status <name>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *JobCommand) Synopsis() string {
	return "Interact with jobs"
}

func (f *JobCommand) Name() string { return "job" }

func (f *JobCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// JobPredictor returns an autocomplete predictor that can be used across
// multiple commands
func JobPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
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
