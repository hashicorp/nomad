package command

import "github.com/mitchellh/cli"

type ACLPolicyCommand struct {
	Meta
}

func (f *ACLPolicyCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *ACLPolicyCommand) Synopsis() string {
	return "Interact with ACL policies"
}

func (f *ACLPolicyCommand) Run(args []string) int {
	return cli.RunResultHelp
}
