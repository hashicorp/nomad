// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/cli"
)

// ensure interface satisfaction
var _ cli.Command = &VolumeClaimListCommand{}

type VolumeClaimDeleteCommand struct {
	Meta
}

func (c *VolumeClaimDeleteCommand) Help() string {
	helpText := `
Usage: nomad volume claims <subcommand> [options]
  volume claims groups commands that interact with volumes claims.
  List existing volume claims:
      $ nomad volume claims list
  Delete an existing volume claim:
      $ nomad volume claims delete <id>
  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeClaimDeleteCommand) Name() string {
	return "volume claims"
}

func (c *VolumeClaimDeleteCommand) Synopsis() string {
	return "Interact with volume claims"
}

func (c *VolumeClaimDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the claim ID to delete
	if len(flags.Args()) != 1 {
		c.Ui.Error("This command takes one argument: <claim_id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	claimID := flags.Args()[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Delete the specified method
	_, err = client.TaskGroupHostVolumeClaims().Delete(claimID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting claim: %s", err))
		return 1
	}

	// Give some feedback to indicate the deletion was successful.
	c.Ui.Output(fmt.Sprintf("Task group host volume claim %s successfully deleted", claimID))
	return 0
}
