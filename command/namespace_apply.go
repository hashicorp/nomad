package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NamespaceApplyCommand struct {
	Meta
}

func (c *NamespaceApplyCommand) Help() string {
	helpText := `
Usage: nomad namespace apply [options]

Apply is used to create or update a namespace.

General Options:

  ` + generalOptionsUsage() + `

Apply Options:

  -name
    The name of the namespace.

  -description
    An optional description for the namespace.
`
	return strings.TrimSpace(helpText)
}

func (c *NamespaceApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":        complete.PredictAnything,
			"-description": complete.PredictAnything,
		})
}

func (c *NamespaceApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NamespaceApplyCommand) Synopsis() string {
	return "Create or update a namespace"
}

func (c *NamespaceApplyCommand) Run(args []string) int {
	var name, description string

	flags := c.Meta.FlagSet("namespace apply", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&description, "description", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Validate we have at-least a name
	if name == "" {
		c.Ui.Error("Namespace name required")
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Create the request object.
	ns := &api.Namespace{
		Name:        name,
		Description: description,
	}

	_, err = client.Namespaces().Register(ns, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying namespace: %s", err))
		return 1
	}

	return 0
}
