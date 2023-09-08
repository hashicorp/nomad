// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
	"github.com/skratchdot/open-golang/open"
)

var (
	// uiContexts is the contexts the ui can open automatically.
	uiContexts = []contexts.Context{contexts.Jobs, contexts.Allocs, contexts.Nodes}
)

type UiCommand struct {
	Meta
}

func (c *UiCommand) Help() string {
	helpText := `
Usage: nomad ui [options] <identifier>

Open the Nomad Web UI in the default browser. An optional identifier may be
provided, in which case the UI will be opened to view the details for that
object. Supported identifiers are jobs, allocations and nodes.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

UI Options

  -authenticate: Exchange your Nomad ACL token for a one-time token in the
    web UI, if ACLs are enabled.

  -show-url: Show the Nomad UI URL instead of opening with the default browser.
`

	return strings.TrimSpace(helpText)
}

func (c *UiCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *UiCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.All, nil)
		if err != nil {
			return []string{}
		}

		final := make([]string, 0)

		for _, allowed := range uiContexts {
			matches, ok := resp.Matches[allowed]
			if !ok {
				continue
			}
			if len(matches) == 0 {
				continue
			}

			final = append(final, matches...)
		}

		return final
	})
}

func (c *UiCommand) Synopsis() string {
	return "Open the Nomad Web UI"
}

func (c *UiCommand) Name() string { return "ui" }

func (c *UiCommand) Run(args []string) int {
	var authenticate bool
	var showUrl bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&authenticate, "authenticate", false, "")
	flags.BoolVar(&showUrl, "show-url", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no more than one argument
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes no or one optional argument, [<identifier>]")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	url, err := url.Parse(client.Address())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing Nomad address %q: %s", client.Address(), err))
		return 1
	}

	// Set query params if necessary
	qp := url.Query()
	if ns := c.clientConfig().Namespace; ns != "" {
		qp.Add("namespace", ns)
	}
	if region := c.clientConfig().Region; region != "" {
		qp.Add("region", region)
	}
	url.RawQuery = qp.Encode()

	// Set one-time secret
	var ottSecret string
	if authenticate {
		ott, _, err := client.ACLTokens().UpsertOneTimeToken(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Could not get one-time token: %s", err))
			return 1
		}
		ottSecret = ott.OneTimeSecretID
	}

	// We were given an id so look it up
	if len(args) == 1 {
		id := args[0]

		// Query for the context associated with the id
		res, _, err := client.Search().PrefixSearch(id, contexts.All, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying search with id: %q", err))
			return 1
		}

		if res.Matches == nil {
			c.Ui.Error(fmt.Sprintf("No matches returned for query: %q", err))
			return 1
		}

		var match contexts.Context
		var fullID string
		matchCount := 0
		for _, ctx := range uiContexts {
			vers, ok := res.Matches[ctx]
			if !ok {
				continue
			}

			if l := len(vers); l == 1 {
				match = ctx
				fullID = vers[0]
				matchCount++
			} else if l > 0 && vers[0] == id {
				// Exact match
				match = ctx
				fullID = vers[0]
				break
			}

			// Only a single result should return, as this is a match against a full id
			if matchCount > 1 || len(vers) > 1 {
				c.logMultiMatchError(id, res.Matches)
				return 1
			}
		}

		switch match {
		case contexts.Nodes:
			url.Path = fmt.Sprintf("ui/clients/%s", fullID)
		case contexts.Allocs:
			url.Path = fmt.Sprintf("ui/allocations/%s", fullID)
		case contexts.Jobs:
			url.Path = fmt.Sprintf("ui/jobs/%s", fullID)
		default:
			c.Ui.Error(fmt.Sprintf("Unable to resolve ID: %q", id))
			return 1
		}
	}

	var output string
	if authenticate && ottSecret != "" {
		output = fmt.Sprintf("Opening URL %q with one-time token", url.String())
		qp := url.Query()
		qp.Add("ott", ottSecret)
		url.RawQuery = qp.Encode()
	} else {
		output = fmt.Sprintf("Opening URL %q", url.String())
	}

	if showUrl {
		c.Ui.Output(fmt.Sprintf("URL for web UI: %s", url.String()))
		return 0
	}

	c.Ui.Output(output)
	if err := open.Start(url.String()); err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening URL: %s", err))
		return 1
	}
	return 0
}

// logMultiMatchError is used to log an error message when multiple matches are
// found. The error message logged displays the matched IDs per context.
func (c *UiCommand) logMultiMatchError(id string, matches map[contexts.Context][]string) {
	c.Ui.Error(fmt.Sprintf("Multiple matches found for id %q", id))
	for _, ctx := range uiContexts {
		vers, ok := matches[ctx]
		if !ok {
			continue
		}
		if len(vers) == 0 {
			continue
		}

		c.Ui.Error(fmt.Sprintf("\n%s:", strings.Title(string(ctx))))
		c.Ui.Error(strings.Join(vers, ", "))
	}
}
