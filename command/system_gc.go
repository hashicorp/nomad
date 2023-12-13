// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SystemGCCommand struct {
	Meta
}

func (c *SystemGCCommand) Help() string {
	helpText := `
Usage: nomad system gc [options]

  Initializes a garbage collection of jobs, evaluations, allocations, and nodes.

  If ACLs are enabled, this option requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)
	return strings.TrimSpace(helpText)
}

func (c *SystemGCCommand) Synopsis() string {
	return "Run the system garbage collection process"
}

func (c *SystemGCCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *SystemGCCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SystemGCCommand) Name() string { return "system gc" }

func (c *SystemGCCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if err := client.System().GarbageCollect(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error running system garbage-collection: %s", err))
		return 1
	}
	return 0
}
