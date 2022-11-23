package command

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"golang.org/x/exp/slices"
)

// Ensure ACLRoleCreateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodCreateCommand{}

// ACLAuthMethodCreateCommand implements cli.Command.
type ACLAuthMethodCreateCommand struct {
	Meta

	name          string
	methodType    string
	tokenLocality string
	maxTokenTTL   time.Duration
	isDefault     bool
	config        string
	json          bool
	tmpl          string
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodCreateCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method create [options]

  Create is used to create new ACL auth methods. Use requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

ACL Auth Method Create Options:

  -name
    Sets the human readable name for the ACL auth method. The name must be
    between 1-128 characters and is a required parameter.

  -type
    Sets the type of the auth method. Currently the only supported type is
    'OIDC'.

  -max-token-ttl
    Sets the duration of time all tokens created by this auth method should be
    valid for.

  -token-locality
    Defines the kind of token that this auth method should produce. This can be
    either 'local' or 'global'. If empty the value of 'local' is assumed.

  -default
    Specifies whether this auth method should be treated as a default one in
    case no auth method is explicitly specified for a login command.

  -config
    Auth method configuration in JSON format.

  -json
    Output the auth method in a JSON format.

  -t
    Format and display the auth method using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":           complete.PredictAnything,
			"-type":           complete.PredictAnything,
			"-max-token-ttl":  complete.PredictAnything,
			"-token-locality": complete.PredictAnything,
			"-default":        complete.PredictAnything,
			"-config":         complete.PredictNothing,
			"-json":           complete.PredictNothing,
			"-t":              complete.PredictAnything,
		})
}

func (a *ACLAuthMethodCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodCreateCommand) Synopsis() string { return "Create a new ACL auth method" }

// Name returns the name of this command.
func (a *ACLAuthMethodCreateCommand) Name() string { return "acl auth-method create" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodCreateCommand) Run(args []string) int {

	flags := a.Meta.FlagSet(a.Name(), FlagSetClient)
	flags.Usage = func() { a.Ui.Output(a.Help()) }
	flags.StringVar(&a.name, "name", "", "")
	flags.StringVar(&a.methodType, "type", "", "")
	flags.StringVar(&a.tokenLocality, "token-locality", "local", "")
	flags.DurationVar(&a.maxTokenTTL, "max-token-ttl", 0, "")
	flags.BoolVar(&a.isDefault, "json", false, "")
	flags.StringVar(&a.config, "config", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments.
	if len(flags.Args()) != 0 {
		a.Ui.Error("This command takes no arguments")
		a.Ui.Error(commandErrorText(a))
		return 1
	}

	// Perform some basic validation
	if a.name == "" {
		a.Ui.Error("ACL auth method name must be specified using the -name flag")
		return 1
	}
	if !slices.Contains([]string{"global", "local"}, a.tokenLocality) {
		a.Ui.Error("Token locality must be set to either 'local' or 'global'")
		return 1
	}
	if strings.ToLower(a.methodType) != "oidc" {
		a.Ui.Error("ACL auth method type must be set to 'OIDC'")
		return 1
	}
	if len(a.config) == 0 || !isJSONString(a.config) {
		a.Ui.Error("Must provide ACL auth method config in JSON format")
		return 1
	}

	configJSON, err := configStringToAuthConfig(a.config)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Unable to parse JSON config: %v", err))
		return 1
	}

	// Set up the auth method with the passed parameters.
	authMethod := api.ACLAuthMethod{
		Name:          a.name,
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

	// Create the auth method via the API.
	method, _, err := client.ACLAuthMethods().Create(&authMethod, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error creating ACL auth method: %s", err))
		return 1
	}

	if a.json || len(a.tmpl) > 0 {
		out, err := Format(a.json, a.tmpl, method)
		if err != nil {
			a.Ui.Error(err.Error())
			return 1
		}

		a.Ui.Output(out)
		return 0
	}

	a.Ui.Output(fmt.Sprintf("Created ACL auth method %s", a.name))
	return 0
}

func configStringToAuthConfig(conf string) (*api.ACLAuthMethodConfig, error) {
	configJSON := api.ACLAuthMethodConfig{}
	err := json.Unmarshal([]byte(conf), &configJSON)

	if err != nil {
		return nil, err
	}

	return &configJSON, nil
}

func isJSONString(s string) bool {
	var js string
	return json.Unmarshal([]byte(s), &js) == nil
}
