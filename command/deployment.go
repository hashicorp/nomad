package command

import "github.com/mitchellh/cli"

type DeploymentCommand struct {
	Meta
}

func (f *DeploymentCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *DeploymentCommand) Synopsis() string {
	return "Interact with deployments"
}

func (f *DeploymentCommand) Run(args []string) int {
	return cli.RunResultHelp
}
