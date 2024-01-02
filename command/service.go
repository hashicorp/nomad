// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ServiceCommand struct {
	Meta
}

func (c *ServiceCommand) Help() string {
	helpText := `
Usage: nomad service <subcommand> [options]

  This command groups subcommands for interacting with the services API.

  List services:

      $ nomad service list

  Detail an individual service:

      $ nomad service info <service_name>

  Delete an individual service registration:

      $ nomad service delete <service_name> <service_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *ServiceCommand) Name() string { return "service" }

func (c *ServiceCommand) Synopsis() string { return "Interact with registered services" }

func (c *ServiceCommand) Run(_ []string) int { return cli.RunResultHelp }
