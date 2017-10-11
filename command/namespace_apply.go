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
Usage: nomad namespace apply [options] <namespace>

  Apply is used to create or update a namespace. It takes the namespace name to
  create or update as its only argument.

General Options:

  ` + generalOptionsUsage() + `

Apply Options:

  -quota
    The quota to attach to the namespace.

  -description
    An optional description for the namespace.
`
	return strings.TrimSpace(helpText)
}

func (c *NamespaceApplyCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-description": complete.PredictAnything,
			"-quota":       QuotaPredictor(c.Meta.Client),
		})
}

func (c *NamespaceApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *NamespaceApplyCommand) Synopsis() string {
	return "Create or update a namespace"
}

func (c *NamespaceApplyCommand) Run(args []string) int {
	var description, quota string

	flags := c.Meta.FlagSet("namespace apply", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&description, "description", "", "")
	flags.StringVar(&quota, "quota", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	name := args[0]

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
		Quota:       quota,
	}

	_, err = client.Namespaces().Register(ns, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying namespace: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied namespace %q!", name))
	return 0
}
