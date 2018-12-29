package command

import "github.com/mitchellh/cli"

type SentinelCommand struct {
	Meta
}

func (f *SentinelCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *SentinelCommand) Synopsis() string {
	return "Interact with Sentinel policies"
}

func (f *SentinelCommand) Run(args []string) int {
	return cli.RunResultHelp
}
