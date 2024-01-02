// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type DeploymentCommand struct {
	Meta
}

func (f *DeploymentCommand) Help() string {
	helpText := `
Usage: nomad deployment <subcommand> [options] [args]

  This command groups subcommands for interacting with deployments. Deployments
  are used to manage a transition between two versions of a Nomad job. Users
  can inspect an ongoing deployment, promote canary allocations, force fail
  deployments, and more.

  Examine a deployments status:

      $ nomad deployment status <deployment-id>

  Promote the canaries to allow the remaining allocations to be updated in a
  rolling deployment fashion:

      $ nomad deployment promote <deployment-id>

  Mark a deployment as failed. This will stop new allocations from being placed
  and if the job's upgrade block specifies auto_revert, causes the job to
  revert back to the last stable version of the job:

      $ nomad deployment fail <deployment-id>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *DeploymentCommand) Synopsis() string {
	return "Interact with deployments"
}

func (f *DeploymentCommand) Name() string { return "deployment" }

func (f *DeploymentCommand) Run(args []string) int {
	return cli.RunResultHelp
}
