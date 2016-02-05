package command

import "github.com/mitchellh/cli"

type FSCommand struct {
	Meta
}

func (f *FSCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *FSCommand) Synopsis() string {
	return "Inspect the contents of an allocation directory"
}

func (f *FSCommand) Run(args []string) int {
	return cli.RunResultHelp
}
