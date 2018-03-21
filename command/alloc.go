package command

import "github.com/mitchellh/cli"

type AllocCommand struct {
	Meta
}

func (f *AllocCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *AllocCommand) Synopsis() string {
	return "Interact with allocations"
}

func (f *AllocCommand) Run(args []string) int {
	return cli.RunResultHelp
}
