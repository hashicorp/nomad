// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
)

// ensure interface satisfaction
var _ cli.Command = &VolumeClaimCommand{}

type VolumeClaimCommand struct {
	Meta
}

func (c *VolumeClaimCommand) Help() string {
	helpText := `
Usage: nomad volume claim <subcommand> [options]

  volume claim groups commands that interact with volumes claims.

  List existing volume claims:
      $ nomad volume claim list

  Delete an existing volume claim:
      $ nomad volume claim delete <id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeClaimCommand) Name() string {
	return "volume claim"
}

func (c *VolumeClaimCommand) Synopsis() string {
	return "Interact with volume claims"
}

func (c *VolumeClaimCommand) Run(args []string) int {
	return cli.RunResultHelp
}
