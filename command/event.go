package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

var _ cli.Command = &EventCommand{}

type EventCommand struct {
	Meta
}

// Help should return long-form help text that includes the command-line
// usage, a brief few sentences explaining the function of the command,
// and the complete list of flags the command accepts.
func (e *EventCommand) Help() string {
	helpText := `
Usage: nomad event <subcommand> [options] [args]

This command has subcommands for interacting with Nomad event sinks.
	`
	return strings.TrimSpace(helpText)
}

// Run should run the actual command with the given CLI instance and
// command-line arguments. It should return the exit status when it is
// finished.
//
// There are a handful of special exit codes this can return documented
// above that change behavior.
func (e *EventCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// Synopsis should return a one-line, short synopsis of the command.
// This should be less than 50 characters ideally.
func (e *EventCommand) Synopsis() string {
	panic("not implemented") // TODO: Implement
}
