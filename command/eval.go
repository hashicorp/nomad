package command

import "github.com/mitchellh/cli"

type EvalCommand struct {
	Meta
}

func (f *EvalCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *EvalCommand) Synopsis() string {
	return "Interact with evaluations"
}

func (f *EvalCommand) Run(args []string) int {
	return cli.RunResultHelp
}
