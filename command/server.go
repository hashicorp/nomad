package command

import "github.com/mitchellh/cli"

type ServerCommand struct {
	Meta
}

func (f *ServerCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *ServerCommand) Synopsis() string {
	return "Interact with servers"
}

func (f *ServerCommand) Run(args []string) int {
	return cli.RunResultHelp
}
