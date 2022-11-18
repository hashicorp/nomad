package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodInfoCommand{}

// ACLAuthMethodInfoCommand implements cli.Command.
type ACLAuthMethodInfoCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodInfoCommand) Help() string {
	helpText := `
  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-by-name": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (a *ACLAuthMethodInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodInfoCommand) Synopsis() string {
	return "Fetch information on an existing ACL auth method"
}

// Name returns the name of this command.
func (a *ACLAuthMethodInfoCommand) Name() string { return "acl role info" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodInfoCommand) Run(args []string) int {

	return 0
}
