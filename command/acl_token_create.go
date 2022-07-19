package command

import (
	"fmt"
	"strings"
	"time"

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

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

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

  -ttl
    Specifies the time-to-live of the created ACL token. This takes the form of
    a time duration such as "5m" and "1h". By default, tokens will be created
    without a TTL and therefore never expire.
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
			"ttl":    complete.PredictAnything,
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
	var name, tokenType, ttl string
	var global bool
	var policies []string
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&tokenType, "type", "client", "")
	flags.BoolVar(&global, "global", false, "")
	flags.StringVar(&ttl, "ttl", "", "")
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

	// If the user set a TTL flag value, convert this to a time duration and
	// add it to our token request object.
	if ttl != "" {
		ttlDuration, err := time.ParseDuration(ttl)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse TTL as time duration: %s", err))
			return 1
		}
		tk.ExpirationTTL = ttlDuration
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
