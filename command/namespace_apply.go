package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	flaghelper "github.com/hashicorp/nomad/helper/flag-helpers"
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

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

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
	return NamespacePredictor(c.Meta.Client, nil)
}

func (c *NamespaceApplyCommand) Synopsis() string {
	return "Create or update a namespace"
}

func (c *NamespaceApplyCommand) Name() string { return "namespace apply" }

func (c *NamespaceApplyCommand) Run(args []string) int {
	var description, quota *string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.Var((flaghelper.FuncVar)(func(s string) error {
		description = &s
		return nil
	}), "description", "")
	flags.Var((flaghelper.FuncVar)(func(s string) error {
		quota = &s
		return nil
	}), "quota", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <namespace>")
		c.Ui.Error(commandErrorText(c))
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

	// Lookup the given namespace
	ns, _, err := client.Namespaces().Info(name, nil)
	if err != nil && !strings.Contains(err.Error(), "404") {
		c.Ui.Error(fmt.Sprintf("Error looking up namespace: %s", err))
		return 1
	}

	if ns == nil {
		ns = &api.Namespace{
			Name: name,
		}
	}

	// Add what is set
	if description != nil {
		ns.Description = *description
	}
	if quota != nil {
		ns.Quota = *quota
	}

	_, err = client.Namespaces().Register(ns, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying namespace: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied namespace %q!", name))
	return 0
}
