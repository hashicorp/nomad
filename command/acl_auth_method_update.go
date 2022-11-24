package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"golang.org/x/exp/slices"
)

// Ensure ACLAuthMethodUpdateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodUpdateCommand{}

// ACLRoleUpdateCommand implements cli.Command.
type ACLAuthMethodUpdateCommand struct {
	Meta

	methodType    string
	tokenLocality string
	maxTokenTTL   time.Duration
	isDefault     bool
	config        string
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodUpdateCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method update [options] <acl_auth_method_name>

  Update is used to update ACL auth methods. Use requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Auth Method Update Options:

  -type
    Updates the type of the auth method. Currently the only supported type is
    'OIDC'.

  -max-token-ttl
    Updates the duration of time all tokens created by this auth method should be
    valid for.

  -token-locality
    Updates the kind of token that this auth method should produce. This can be
    either 'local' or 'global'. If empty the value of 'local' is assumed.

  -default
    Specifies whether this auth method should be treated as a default one in
    case no auth method is explicitly specified for a login command.

  -config
    Updates auth method configuration (in JSON format).
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodUpdateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":           complete.PredictAnything,
			"-max-token-ttl":  complete.PredictAnything,
			"-token-locality": complete.PredictAnything,
			"-default":        complete.PredictAnything,
			"-config":         complete.PredictNothing,
		})
}

func (a *ACLAuthMethodUpdateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodUpdateCommand) Synopsis() string { return "Update an existing ACL auth method" }

// Name returns the name of this command.
func (*ACLAuthMethodUpdateCommand) Name() string { return "acl auth-method update" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodUpdateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.methodType, "type", "", "")
	flags.StringVar(&a.tokenLocality, "token-locality", "local", "")
	flags.DurationVar(&a.maxTokenTTL, "max-token-ttl", 0, "")
	flags.StringVar(&a.config, "config", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the auth method name to delete.
	if len(flags.Args()) != 1 {
		a.Ui.Error("This command takes one argument: <acl_auth_method_name>")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	originalMethodName := flags.Args()[0]

	if a.tokenLocality != "" && !slices.Contains([]string{"global", "local"}, a.tokenLocality) {
		a.Ui.Error("Token locality must be set to either 'local' or 'global'")
		return 1
	}
	if a.methodType != "" && strings.ToLower(a.methodType) != "oidc" {
		a.Ui.Error("ACL auth method type must be set to 'OIDC'")
		return 1
	}

	var configJSON *api.ACLAuthMethodConfig
	if len(a.config) != 0 {
		var err error
		configJSON, err = configStringToAuthConfig(a.config)
		if err != nil {
			a.Ui.Error(fmt.Sprintf("Unable to parse JSON config: %v", err))
			return 1
		}
	}

	// Set up the auth method with the passed parameters.
	authMethod := api.ACLAuthMethod{
		Name:          originalMethodName,
		Type:          a.methodType,
		TokenLocality: a.tokenLocality,
		MaxTokenTTL:   a.maxTokenTTL,
		Default:       a.isDefault,
		Config:        configJSON,
	}

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Update the auth method via the API.
	_, err = client.ACLAuthMethods().Update(&authMethod, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error updating ACL auth method: %v", err))
		return 1
	}

	a.Ui.Output(fmt.Sprintf("Updated ACL auth method %s", originalMethodName))
	return 0
}
