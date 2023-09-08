// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ServerCommand struct {
	Meta
}

func (f *ServerCommand) Help() string {
	helpText := `
Usage: nomad server <subcommand> [options] [args]

  This command groups subcommands for interacting with Nomad servers. Users can
  list Servers, join a server to the cluster, and force leave a server.

  List Nomad servers:

      $ nomad server members

  Join a new server to another:

      $ nomad server join "IP:Port"

  Force a server to leave:

      $ nomad server force-leave <name>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *ServerCommand) Synopsis() string {
	return "Interact with servers"
}

func (f *ServerCommand) Name() string { return "server" }

func (f *ServerCommand) Run(args []string) int {
	return cli.RunResultHelp
}
