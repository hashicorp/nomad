// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"io"
	"strings"

	"github.com/posener/complete"
)

const defaultEscapeChar = "~"

type ActionCommand struct {
	Meta

	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func (c *ActionCommand) Help() string {
	helpText := `
Usage: nomad action [options] <action>

  Perform a predefined command inside the environment of a given context.
  Currently this acts as a wrapper around the job action command.

  When ACLs are enabled, this command requires a token with the 'alloc-exec',
  'read-job', and 'list-jobs' capabilities for a task's namespace. If
  the task driver does not have file system isolation (as with 'raw_exec'),
  this command requires the 'alloc-node-exec', 'read-job', and 'list-jobs'
  capabilities for the task's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *ActionCommand) Synopsis() string {
	return "Run a pre-defined command from a given context"
}

func (c *ActionCommand) AutocompleteFlags() complete.Flags {
	return nil
}

func (c *ActionCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ActionCommand) Name() string { return "action" }

func (c *ActionCommand) Run(args []string) int {

	cmd := &JobActionCommand{
		Meta:   c.Meta,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}

	return cmd.Run(args)
}
