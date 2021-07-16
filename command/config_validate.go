package command

import (
	"fmt"
	"strings"

	agent "github.com/hashicorp/nomad/command/agent"
)

type ConfigValidateCommand struct {
	Meta
}

func (c *ConfigValidateCommand) Help() string {
	helpText := `
Usage: nomad config validate <config_path> [<config_path...>]

  Performs a thorough sanity test on Nomad configuration files. For each file
  or directory given, the validate command will attempt to parse the contents
  just as the "nomad agent" command would, and catch any errors.

  This is useful to do a test of the configuration only, without actually
  starting the agent. This performs all of the validation the agent would, so
  this should be given the complete set of configuration files that are going
  to be loaded by the agent. This command cannot operate on partial
  configuration fragments since those won't pass the full agent validation.

  Returns 0 if the configuration is valid, or 1 if there are problems.
`

	return strings.TrimSpace(helpText)
}

func (c *ConfigValidateCommand) Synopsis() string {
	return "Validate config files/directories"
}

func (c *ConfigValidateCommand) Name() string { return "config validate" }

func (c *ConfigValidateCommand) Run(args []string) int {
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

	var config *agent.Config

	for _, path := range configPath {
		current, err := agent.LoadConfig(path)
		if err != nil {
			c.Ui.Error(fmt.Sprintf(
				"Error loading configuration from %s: %s", path, err))
			return 1
		}

		if config == nil {
			config = current
		} else {
			config = config.Merge(current)
		}
	}

	c.Ui.Output("Configuration is valid!")
	return 0
}
