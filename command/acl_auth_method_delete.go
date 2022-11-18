package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodDeleteCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodDeleteCommand{}

// ACLAuthMethodDeleteCommand implements cli.Command.
type ACLAuthMethodDeleteCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodDeleteCommand) Help() string {
	helpText := `

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (a *ACLAuthMethodDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodDeleteCommand) Synopsis() string { return "Delete an existing ACL role" }

// Name returns the name of this command.
func (a *ACLAuthMethodDeleteCommand) Name() string { return "acl token delete" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodDeleteCommand) Run(args []string) int {

	return 0
}
