package command

import "github.com/mitchellh/cli"

type ACLCommand struct {
	Meta
}

func (f *ACLCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *ACLCommand) Synopsis() string {
	return "Interact with ACL policies and tokens"
}

func (f *ACLCommand) Run(args []string) int {
	return cli.RunResultHelp
}
