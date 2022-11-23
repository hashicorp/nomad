package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodInfoCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodInfoCommand{}

// ACLAuthMethodInfoCommand implements cli.Command.
type ACLAuthMethodInfoCommand struct {
	Meta

	json bool
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodInfoCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method info [options] <acl_method_name>

  Info is used to fetch information on an existing ACL auth method. Requires a
  management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Info Options:

  -json
    Output the ACL role in a JSON format.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
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
func (a *ACLAuthMethodInfoCommand) Name() string { return "acl auth-method info" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodInfoCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.BoolVar(&a.json, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have exactly one argument.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_auth_method_name>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	var (
		aclRole *api.ACLAuthMethod
		apiErr  error
	)

	methodName := flags.Args()[0]

	method, _, apiErr = client.ACLAuthMethods().GetByName(methodName, nil)

	// Handle any error from the API.
	if apiErr != nil {
		a.Ui.Error(fmt.Sprintf("Error reading ACL auth method: %s", apiErr))
		return 1
	}

	// Format the output.
	a.Ui.Output(formatAuthMethod(method))

	return 0
}
