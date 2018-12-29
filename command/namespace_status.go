package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NamespaceStatusCommand struct {
	Meta
}

func (c *NamespaceStatusCommand) Help() string {
	helpText := `
Usage: nomad namespace status [options] <namespace>

  Status is used to view the status of a particular namespace.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *NamespaceStatusCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *NamespaceStatusCommand) AutocompleteArgs() complete.Predictor {
	return NamespacePredictor(c.Meta.Client, nil)
}

func (c *NamespaceStatusCommand) Synopsis() string {
	return "Display a namespace's status"
}

func (c *NamespaceStatusCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("namespace status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	name := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Do a prefix lookup
	ns, possible, err := getNamespace(client.Namespaces(), name)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving namespaces: %s", err))
		return 1
	}

	if len(possible) != 0 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple namespaces\n\n%s", formatNamespaces(possible)))
		return 1
	}

	c.Ui.Output(formatNamespaceBasics(ns))

	if ns.Quota != "" {
		quotas := client.Quotas()
		spec, _, err := quotas.Info(ns.Quota, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving quota spec: %s", err))
			return 1
		}

		// Get the quota usages
		usages, failures := quotaUsages(spec, quotas)

		// Format the limits
		c.Ui.Output(c.Colorize().Color("\n[bold]Quota Limits[reset]"))
		c.Ui.Output(formatQuotaLimits(spec, usages))

		// Display any failures
		if len(failures) != 0 {
			c.Ui.Error(c.Colorize().Color("\n[bold][red]Lookup Failures[reset]"))
			for region, failure := range failures {
				c.Ui.Error(fmt.Sprintf("  * Failed to retrieve quota usage for region %q: %v", region, failure))
				return 1
			}
		}
	}

	return 0
}

// formatNamespaceBasics formats the basic information of the namespace
func formatNamespaceBasics(ns *api.Namespace) string {
	basic := []string{
		fmt.Sprintf("Name|%s", ns.Name),
		fmt.Sprintf("Description|%s", ns.Description),
		fmt.Sprintf("Quota|%s", ns.Quota),
	}

	return formatKV(basic)
}

func getNamespace(client *api.Namespaces, ns string) (match *api.Namespace, possible []*api.Namespace, err error) {
	// Do a prefix lookup
	namespaces, _, err := client.PrefixList(ns, nil)
	if err != nil {
		return nil, nil, err
	}

	l := len(namespaces)
	switch {
	case l == 0:
		return nil, nil, fmt.Errorf("Namespace %q matched no namespaces", ns)
	case l == 1:
		return namespaces[0], nil, nil
	default:
		return nil, namespaces, nil
	}
}
