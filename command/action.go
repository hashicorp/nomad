// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"strings"

	"github.com/posener/complete"
)

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

  ` + generalOptionsUsage(usageOptsNoNamespace) + `

Action Specific Options:

  -job <job-id>
    Specifies the job in which the Action is defined

  -alloc <allocation-id>
    Specifies the allocation in which the Action is defined. If not provided,
    a group and task name must be provided and a random allocation will be
    selected from the job.

  -task <task-name>
    Specifies the task in which the Action is defined. Required if no
    allocation is provided.

  -group=<group-name>
    Specifies the group in which the Action is defined. Required if no
    allocation is provided.

  -i
    Pass stdin to the container, defaults to true. Pass -i=false to disable.

  -t
    Allocate a pseudo-tty, defaults to true if stdin is detected to be a tty session.
    Pass -t=false to disable explicitly.

  -e <escape_char>
    Sets the escape character for sessions with a pty (default: '~'). The escape
    character is only recognized at the beginning of a line. The escape character
    followed by a dot ('.') closes the connection. Setting the character to
    'none' disables any escapes and makes the session fully transparent.
`
	return strings.TrimSpace(helpText)
}

func (c *ActionCommand) Synopsis() string {
	return "Run a pre-defined command from a given context"
}

func (c *ActionCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-job":   complete.PredictAnything,
			"-alloc": complete.PredictAnything,
			"-task":  complete.PredictAnything,
			"-group": complete.PredictAnything,
			"-i":     complete.PredictNothing,
			"-t":     complete.PredictNothing,
			"-e":     complete.PredictAnything,
		})
}

func (c *ActionCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ActionCommand) Name() string { return "action" }

const defaultEscapeChar = "~"

func (c *ActionCommand) Run(args []string) int {

	var stdinOpt, ttyOpt bool
	var task, allocation, job, group, escapeChar string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&task, "task", "", "")
	flags.StringVar(&group, "group", "", "")
	flags.StringVar(&allocation, "alloc", "", "")
	flags.StringVar(&job, "job", "", "")
	flags.BoolVar(&stdinOpt, "i", true, "")
	flags.BoolVar(&ttyOpt, "t", isTty(), "")
	flags.StringVar(&escapeChar, "e", defaultEscapeChar, "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	args = flags.Args()

	if len(args) < 1 {
		c.Ui.Error("An action name is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var commandWithFlags []string

	if task != "" {
		commandWithFlags = append(commandWithFlags, "-task="+task)
	}
	if group != "" {
		commandWithFlags = append(commandWithFlags, "-group="+group)
	}
	if allocation != "" {
		commandWithFlags = append(commandWithFlags, "-alloc="+allocation)
	}
	if job != "" {
		commandWithFlags = append(commandWithFlags, "-job="+job)
	}

	if !stdinOpt {
		commandWithFlags = append(commandWithFlags, "-i=false")
	}
	if !ttyOpt {
		commandWithFlags = append(commandWithFlags, "-t=false")
	}
	if escapeChar != defaultEscapeChar {
		commandWithFlags = append(commandWithFlags, "-e="+escapeChar)
	}

	commandWithFlags = append(commandWithFlags, args...)

	cmd := &JobActionCommand{
		Meta:   c.Meta,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	}

	return cmd.Run(commandWithFlags)
}
