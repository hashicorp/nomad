package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorAutopilotCommand struct {
	Meta
}

func (c *OperatorAutopilotCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *OperatorAutopilotCommand) Synopsis() string {
	return "Provides tools for modifying Autopilot configuration"
}

func (c *OperatorAutopilotCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot <subcommand> [options]

  This command groups subcommands for interacting with Nomad's Autopilot
  subsystem. Autopilot provides automatic, operator-friendly management of Nomad
  servers. The command can be used to view or modify the current Autopilot
  configuration.

  Get the current Autopilot configuration:

      $ nomad operator autopilot get-config
  
  Set a new Autopilot configuration, enabling automatic dead server cleanup:

      $ nomad operator autopilot set-config -cleanup-dead-servers=true
  
  Please see the individual subcommand help for detailed usage information.
  `
	return strings.TrimSpace(helpText)
	/*
			helpText := `
		Usage: nomad deployment <subcommand> [options] [args]

		  This command groups subcommands for interacting with deployments. Deployments
		  are used to manage a transistion between two versions of a Nomad job. Users
		  can inspect an ongoing deployment, promote canary allocations, force fail
		  deployments, and more.

		  Examine a deployments status:

		      $ nomad deployment status <deployment-id>

		  Promote the canaries to allow the remaining allocations to be updated in a
		  rolling deployment fashion:

		      $ nomad deployment promote <depoloyment-id>

		  Mark a deployment as failed. This will stop new allocations from being placed
		  and if the job's upgrade stanza specifies auto_revert, causes the job to
		  revert back to the last stable version of the job:

		      $ nomad deployment fail <depoloyment-id>

		  Please see the individual subcommand help for detailed usage information.
		`

			return strings.TrimSpace(helpText)
	*/
}
