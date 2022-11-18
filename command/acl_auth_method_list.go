package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodListCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodListCommand{}

// ACLAuthMethodListCommand implements cli.Command.
type ACLAuthMethodListCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodListCommand) Help() string {
	helpText := `
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLAuthMethodListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodListCommand) Synopsis() string { return "List ACL roles" }

// Name returns the name of this command.
func (a *ACLAuthMethodListCommand) Name() string { return "acl role list" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodListCommand) Run(args []string) int {
	return 0
}
