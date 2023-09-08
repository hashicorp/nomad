// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

  This command groups subcommands for interacting with Nomad event sinks.
  Nomad's event sinks system can be used to subscribe to the event stream for
  events that match specific topics.

  Register or update an event sink:

      $ cat sink.json
      {
        "ID": "my-sink",
        "Type": "webhook",
        "Address": "http://127.0.0.1:8080",
        "Topics": {
          "*": ["*"]
        }
      }
      $ nomad event sink register sink.json
      Successfully registered "my-sink" event sink!

  List event sinks:

      $ nomad event sink list
      ID         Type     Address           Topics    LatestIndex
      my-sink    webhook  http://127.0.0.1  *[*]      0

  Deregister an event sink:

      $ nomad event sink deregister my-sink
      Successfully deregistered "my-sink" event sink!

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (e *EventCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (e *EventCommand) Synopsis() string {
	return "Interact with event sinks"
}
