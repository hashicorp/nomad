package command

import "github.com/mitchellh/cli"

type NamespaceCommand struct {
	Meta
}

func (f *NamespaceCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *NamespaceCommand) Synopsis() string {
	return "Interact with namespaces"
}

func (f *NamespaceCommand) Run(args []string) int {
	return cli.RunResultHelp
}
