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

  -

`
	return strings.TrimSpace(helpText)
}

func (c *ACLTokenCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLTokenCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenCreateCommand) Synopsis() string {
	return "Create a new ACL token"
}

func (c *ACLTokenCreateCommand) Run(args []string) int {
	var name, tokenType string
	var global bool
	var policies []string
	flags := c.Meta.FlagSet("acl token create", FlagSetClient)
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
		c.Ui.Error(c.Help())
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

	// Get the bootstrap token
	token, _, err := client.ACLTokens().Create(tk, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error creating token: %s", err))
		return 1
	}

	// Format the output
	c.Ui.Output(formatKVACLToken(token))
	return 0
}
