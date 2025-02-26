// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/cli"
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
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
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
		if !c.askQuestion(fmt.Sprintf("Are you sure you want to delete task group host volume claim %s? [Y/n]", claimID)) {
			c.Ui.Warn(`
If you delete a volume claim, the allocation that uses this claim to "stick" to
this particular volume ID will no longer use it upon its next reschedule or
migration. The deployment of the task group the allocation runs will remain
stateful and grab the next feasible volume ID during reschedule or replacement.
Deleting the volume claim is thus a temporary release of the volume ID, meant
to be used in "emergency" situations, i.e., when the operator wants to drain
the node the volume is on, but does not want to re-deploy the job.

Please use with caution!

`)
			return 0
		}
	}

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

// askQuestion asks question to user until they provide a valid response.
func (c *VolumeClaimDeleteCommand) askQuestion(question string) bool {
	for {
		answer, err := c.Ui.Ask(c.Colorize().Color(fmt.Sprintf("[?] %s", question)))
		if err != nil {
			if err.Error() != "interrupted" {
				c.Ui.Output(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "", "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			c.Ui.Output(fmt.Sprintf(`%q is not a valid response, please answer "yes" or "no".`, answer))
			continue
		}
	}
}
