package command

import "github.com/mitchellh/cli"

type JobCommand struct {
	Meta
}

func (f *JobCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *JobCommand) Synopsis() string {
	return "Interact with jobs"
}

func (f *JobCommand) Run(args []string) int {
	return cli.RunResultHelp
}
