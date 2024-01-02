// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type ACLTokenListCommand struct {
	Meta
}

func (c *ACLTokenListCommand) Help() string {
	helpText := `
Usage: nomad acl token list

  List is used to list existing ACL tokens.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

List Options:

  -json
    Output the ACL tokens in a JSON format.

  -t
    Format and display the ACL tokens using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *ACLTokenListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *ACLTokenListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenListCommand) Synopsis() string {
	return "List ACL tokens"
}

func (c *ACLTokenListCommand) Name() string { return "acl token list" }

func (c *ACLTokenListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch info on the policy
	tokens, _, err := client.ACLTokens().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing ACL tokens: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, tokens)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatTokens(tokens))
	return 0
}

func formatTokens(tokens []*api.ACLTokenListStub) string {
	if len(tokens) == 0 {
		return "No tokens found"
	}

	output := make([]string, 0, len(tokens)+1)
	output = append(output, "Name|Type|Global|Accessor ID|Expired")
	for _, p := range tokens {
		expired := false
		if p.ExpirationTime != nil && !p.ExpirationTime.IsZero() {
			if p.ExpirationTime.Before(time.Now().UTC()) {
				expired = true
			}
		}

		output = append(output, fmt.Sprintf(
			"%s|%s|%t|%s|%v", p.Name, p.Type, p.Global, p.AccessorID, expired))
	}

	return formatList(output)
}
