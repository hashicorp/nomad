// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
)

// ensure interface satisfaction
var _ cli.Command = &VolumeClaimDeleteCommand{}

type VolumeClaimDeleteCommand struct {
	Meta

	autoYes bool
}

func (c *VolumeClaimDeleteCommand) Help() string {
	helpText := `
Usage: nomad volume claim delete <id>

  volume claim delete is used to delete existing host volume claim by claim ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Delete options:

  -y
    Automatically answers "yes" to all the questions, making the deletion
    non-interactive. Defaults to "false".

`
	return strings.TrimSpace(helpText)
}

func (c *VolumeClaimDeleteCommand) Name() string {
	return "volume claim delete"
}

func (c *VolumeClaimDeleteCommand) Synopsis() string {
	return "Delete existing volume claim"
}

func (c *VolumeClaimDeleteCommand) Run(args []string) int {
	flags := c.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.autoYes, "y", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the claim ID to delete
	if len(flags.Args()) != 1 {
		c.Ui.Error("This command takes one argument: <claim_id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if !isTty() && !c.autoYes {
		c.Ui.Error("This command requires -y option when running in non-interactive mode")
		return 1
	}

	claimID := flags.Args()[0]

	if !c.autoYes {
		c.Ui.Warn(`
If you delete a volume claim, the allocation that uses this claim to "stick"
to a particular volume ID will no longer use it upon its next reschedule or
migration. The deployment of the task group the allocation runs will still
claim another feasible volume ID during reschedule or replacement.
`)
		if !c.askQuestion(fmt.Sprintf("Are you sure you want to delete task group host volume claim %s? [Y/n]", claimID)) {
			return 0
		}
	}

	// Get the HTTP client
	client, err := c.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if len(claimID) == shortId {
		claimID = sanitizeUUIDPrefix(claimID)
		claims, _, err := client.TaskGroupHostVolumeClaims().List(nil, &api.QueryOptions{Prefix: claimID})
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying claims: %s", err))
			return 1
		}
		// Return error if no claims are found
		if len(claims) == 0 {
			c.Ui.Error(fmt.Sprintf("No claim(s) with prefix %q found", claimID))
			return 1
		}
		if len(claims) > 1 {
			// Dump the output
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple claims\n\n%s", formatClaims(claims, fullId)))
			return 1
		}
		claimID = claims[0].ID
	}

	// Delete the specified claim
	_, err = client.TaskGroupHostVolumeClaims().Delete(claimID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting claim: %s", err))
		return 1
	}

	// Give some feedback to indicate the deletion was successful.
	c.Ui.Output(fmt.Sprintf("Task group host volume claim %s successfully deleted", claimID))
	return 0
}
