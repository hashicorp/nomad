// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type NamespaceDeleteCommand struct {
	Meta
}

func (c *NamespaceDeleteCommand) Help() string {
	helpText := `
Usage: nomad namespace delete [options] <namespace>

  Delete is used to remove a namespace.

  If ACLs are enabled, this command requires a management ACL token.

  You cannot delete a namespace that has non-terminal jobs. In federated
  clusters, you cannot delete a namespace that has non-terminal jobs in any of
  the federated regions.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *NamespaceDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *NamespaceDeleteCommand) AutocompleteArgs() complete.Predictor {
	filter := map[string]struct{}{"default": {}}
	return NamespacePredictor(c.Meta.Client, filter)
}

func (c *NamespaceDeleteCommand) Synopsis() string {
	return "Delete a namespace"
}

func (c *NamespaceDeleteCommand) Name() string { return "namespace delete" }

func (c *NamespaceDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <namespace>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	namespace := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.Namespaces().Delete(namespace, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting namespace: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted namespace %q!", namespace))
	return 0
}
