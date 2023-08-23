// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ACLAuthMethodUpdateCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodUpdateCommand{}

// ACLAuthMethodUpdateCommand implements cli.Command.
type ACLAuthMethodUpdateCommand struct {
	Meta

	methodType    string
	tokenLocality string
	maxTokenTTL   time.Duration
	isDefault     bool
	config        string
	json          bool
	tmpl          string

	testStdin io.Reader
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
    Updates the type of the auth method. Supported types are 'OIDC' and 'JWT'.

  -max-token-ttl
    Updates the duration of time all tokens created by this auth method should be
    valid for.

  -token-locality
    Updates the kind of token that this auth method should produce. This can be
    either 'local' or 'global'.

  -default
    Specifies whether this auth method should be treated as a default one in
    case no auth method is explicitly specified for a login command.

  -config
    Updates auth method configuration (in JSON format). May be prefixed with
    '@' to indicate that the value is a file path to load the config from. '-'
    may also be given to indicate that the config is available on stdin.

  -json
    Output the ACL auth-method in a JSON format.

  -t
    Format and display the ACL auth-method using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (a *ACLAuthMethodUpdateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(a.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-type":           complete.PredictSet("OIDC", "JWT"),
			"-max-token-ttl":  complete.PredictAnything,
			"-token-locality": complete.PredictSet("local", "global"),
			"-default":        complete.PredictSet("true", "false"),
			"-config":         complete.PredictNothing,
			"-json":           complete.PredictNothing,
			"-t":              complete.PredictAnything,
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
	flags.StringVar(&a.tokenLocality, "token-locality", "", "")
	flags.DurationVar(&a.maxTokenTTL, "max-token-ttl", 0, "")
	flags.StringVar(&a.config, "config", "", "")
	flags.BoolVar(&a.isDefault, "default", false, "")
	flags.BoolVar(&a.json, "json", false, "")
	flags.StringVar(&a.tmpl, "t", "", "")
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

	// Get the HTTP client.
	client, err := a.Meta.Client()
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the method we want to update exists
	originalMethod, _, err := client.ACLAuthMethods().Get(originalMethodName, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error when retrieving ACL auth method: %v", err))
		return 1
	}

	// Check if any command-specific flags were set
	setFlags := []string{}
	for _, f := range []string{"type", "token-locality", "max-token-ttl", "config", "default"} {
		if flagPassed(flags, f) {
			setFlags = append(setFlags, f)
		}
	}
	if len(setFlags) == 0 {
		a.Ui.Error("Please provide at least one flag to update the ACL auth method")
		return 1
	}

	updatedMethod := *originalMethod

	if slices.Contains(setFlags, "token-locality") {
		if !slices.Contains([]string{"global", "local"}, a.tokenLocality) {
			a.Ui.Error("Token locality must be set to either 'local' or 'global'")
			return 1
		}
		updatedMethod.TokenLocality = a.tokenLocality
	}

	if slices.Contains(setFlags, "type") {
		if !slices.Contains([]string{"OIDC", "JWT"}, strings.ToUpper(a.methodType)) {
			a.Ui.Error("ACL auth method type must be set to 'OIDC' or 'JWT'")
			return 1
		}
		updatedMethod.Type = a.methodType
	}

	if slices.Contains(setFlags, "max-token-ttl") {
		if a.maxTokenTTL < 1 {
			a.Ui.Error("Max token TTL must be set to a value between min and max TTL configured for the server.")
			return 1
		}
		updatedMethod.MaxTokenTTL = a.maxTokenTTL
	}

	if slices.Contains(setFlags, "default") {
		updatedMethod.Default = a.isDefault
	}

	if len(a.config) != 0 {
		config, err := loadDataSource(a.config, a.testStdin)
		if err != nil {
			a.Ui.Error(fmt.Sprintf("Error loading configuration: %v", err))
			return 1
		}

		configJSON := api.ACLAuthMethodConfig{}
		err = json.Unmarshal([]byte(config), &configJSON)
		if err != nil {
			a.Ui.Error(fmt.Sprintf("Unable to parse config: %v", err))
			return 1
		}
		updatedMethod.Config = &configJSON
	}

	// Update the auth method via the API.
	method, _, err := client.ACLAuthMethods().Update(&updatedMethod, nil)
	if err != nil {
		a.Ui.Error(fmt.Sprintf("Error updating ACL auth method: %v", err))
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

	outputAuthMethod(a.Meta, method)
	return 0
}

func flagPassed(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
