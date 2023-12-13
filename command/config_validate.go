// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"reflect"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	agent "github.com/hashicorp/nomad/command/agent"
)

type ConfigValidateCommand struct {
	Meta
}

func (c *ConfigValidateCommand) Help() string {
	helpText := `
Usage: nomad config validate <config_path> [<config_path...>]

  Perform validation on a set of Nomad configuration files. This is useful
  to test the Nomad configuration without starting the agent.

  Accepts the path to either a single config file or a directory of
  config files to use for configuring the Nomad agent. This option may
  be specified multiple times. If multiple config files are used, the
  values from each will be merged together. During merging, values from
  files found later in the list are merged over values from previously
  parsed files.

  This command cannot operate on partial configuration fragments since
  those won't pass the full agent validation. This command does not
  require an ACL token.

  Returns 0 if the configuration is valid, or 1 if there are problems.
`

	return strings.TrimSpace(helpText)
}

func (c *ConfigValidateCommand) Synopsis() string {
	return "Validate config files/directories"
}

func (c *ConfigValidateCommand) Name() string { return "config validate" }

func (c *ConfigValidateCommand) Run(args []string) int {
	var mErr multierror.Error
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	configPath := flags.Args()
	if len(configPath) < 1 {
		c.Ui.Error("Must specify at least one config file or directory")
		return 1
	}

	config := agent.DefaultConfig()

	for _, path := range configPath {
		fc, err := agent.LoadConfig(path)
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf(
				"Error loading configuration from %s: %s", path, err))
			continue
		}
		if fc == nil || reflect.DeepEqual(fc, &agent.Config{}) {
			c.Ui.Warn(fmt.Sprintf("No configuration loaded from %s", path))
		}

		config = config.Merge(fc)
	}
	if err := mErr.ErrorOrNil(); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	cmd := agent.Command{Ui: c.Ui}
	valid := cmd.IsValidConfig(config, agent.DefaultConfig())
	if !valid {
		c.Ui.Error("Configuration is invalid")
		return 1
	}

	c.Ui.Output("Configuration is valid!")
	return 0
}
