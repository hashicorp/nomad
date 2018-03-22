package command

import "github.com/mitchellh/cli"

type NodeCommand struct {
	Meta
}

func (f *NodeCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *NodeCommand) Synopsis() string {
	return "Interact with nodes"
}

func (f *NodeCommand) Run(args []string) int {
	return cli.RunResultHelp
}
