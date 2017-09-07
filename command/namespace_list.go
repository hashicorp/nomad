package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NamespaceListCommand struct {
	Meta
}

func (c *NamespaceListCommand) Help() string {
	helpText := `
Usage: nomad namespace list [options]

List is used to list available namespaces.

General Options:

  ` + generalOptionsUsage() + `

List Options:

  -json
    Output the namespaces in a JSON format.

  -t
    Format and display the namespaces using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *NamespaceListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *NamespaceListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NamespaceListCommand) Synopsis() string {
	return "List namespaces"
}

func (c *NamespaceListCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet("namespace list", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	namespaces, _, err := client.Namespaces().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving namespaces: %s", err))
		return 1
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, namespaces)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatNamespaces(namespaces))
	return 0
}

func formatNamespaces(namespaces []*api.Namespace) string {
	if len(namespaces) == 0 {
		return "No namespaces found"
	}

	rows := make([]string, len(namespaces)+1)
	rows[0] = "Name|Description"
	for i, ns := range namespaces {
		rows[i+1] = fmt.Sprintf("%s|%s",
			ns.Name,
			ns.Description)
	}
	return formatList(rows)
}
