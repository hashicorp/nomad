// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import "github.com/mitchellh/cli"

type PluginCommand struct {
	Meta
}

func (c *PluginCommand) Help() string {
	helpText := `
Usage nomad plugin status [options] [plugin]

    This command groups subcommands for interacting with plugins.
`
	return helpText
}

func (c *PluginCommand) Synopsis() string {
	return "Inspect plugins"
}

func (c *PluginCommand) Name() string { return "plugin" }

func (c *PluginCommand) Run(args []string) int {
	return cli.RunResultHelp
}
