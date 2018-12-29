package command

import "github.com/mitchellh/cli"

type ACLTokenCommand struct {
	Meta
}

func (f *ACLTokenCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *ACLTokenCommand) Synopsis() string {
	return "Interact with ACL tokens"
}

func (f *ACLTokenCommand) Run(args []string) int {
	return cli.RunResultHelp
}
