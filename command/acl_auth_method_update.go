package command

import (
	"strings"

	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodUpdateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodUpdateCommand{}

// ACLRoleUpdateCommand implements cli.Command.
type ACLAuthMethodUpdateCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodUpdateCommand) Help() string {
	helpText := `
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodUpdateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name": complete.PredictAnything,
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (a *ACLAuthMethodUpdateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodUpdateCommand) Synopsis() string { return "Update an existing ACL role" }

// Name returns the name of this command.
func (*ACLAuthMethodUpdateCommand) Name() string { return "acl role update" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodUpdateCommand) Run(args []string) int {

	return 0
}
