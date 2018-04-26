package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type ACLTokenCreateCommand struct {
	Meta
}

func (c *ACLTokenCreateCommand) Help() string {
	helpText := `
Usage: nomad acl token create [options]

  Create is used to issue new ACL tokens. Requires a management token.

General Options:

  ` + generalOptionsUsage() + `

Create Options:

  -name=""
    Sets the human readable name for the ACL token.

  -type="client"
    Sets the type of token. Must be one of "client" (default), or "management".

  -global=false
    Toggles the global mode of the token. Global tokens are replicated to all regions.

  -policy=""
    Specifies a policy to associate with the token. Can be specified multiple times,
    but only with client type tokens.
`
	return strings.TrimSpace(helpText)
}

func (c *ACLTokenCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"name":   complete.PredictAnything,
			"type":   complete.PredictAnything,
			"global": complete.PredictNothing,
			"policy": complete.PredictAnything,
		})
}

func (c *ACLTokenCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenCreateCommand) Synopsis() string {
	return "Create a new ACL token"
}

func (c *ACLTokenCreateCommand) Name() string { return "acl token create" }

func (c *ACLTokenCreateCommand) Run(args []string) int {
	var name, tokenType string
	var global bool
	var policies []string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&tokenType, "type", "client", "")
	flags.BoolVar(&global, "global", false, "")
	flags.Var((funcVar)(func(s string) error {
		policies = append(policies, s)
		return nil
	}), "policy", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Setup the token
	tk := &api.ACLToken{
		Name:     name,
		Type:     tokenType,
		Policies: policies,
		Global:   global,
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Create the bootstrap token
	token, _, err := client.ACLTokens().Create(tk, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating token: %s", err))
		return 1
	}

	// Format the output
	c.Ui.Output(formatKVACLToken(token))
	return 0
}
