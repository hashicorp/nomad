// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
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

  If ACLs are enabled, this command requires a management ACL token or a token
  that has a capability associated with the namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Status Specific Options:

  -json
    Output the latest namespace status information in a JSON format.

  -t
    Format and display namespace status information using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *NamespaceStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *NamespaceStatusCommand) AutocompleteArgs() complete.Predictor {
	return NamespacePredictor(c.Meta.Client, nil)
}

func (c *NamespaceStatusCommand) Synopsis() string {
	return "Display a namespace's status"
}

func (c *NamespaceStatusCommand) Name() string { return "namespace status" }

func (c *NamespaceStatusCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <namespace>")
		c.Ui.Error(commandErrorText(c))
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

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, ns)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	c.Ui.Output(formatNamespaceBasics(ns))

	if len(ns.Meta) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Metadata[reset]"))
		var meta []string
		for k := range ns.Meta {
			meta = append(meta, fmt.Sprintf("%s|%s", k, ns.Meta[k]))
		}
		sort.Strings(meta)
		c.Ui.Output(formatKV(meta))
	}

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

	if ns.NodePoolConfiguration != nil {
		c.Ui.Output(c.Colorize().Color("\n[bold]Node Pool Configuration[reset]"))
		npConfig := ns.NodePoolConfiguration
		npConfigOut := []string{
			fmt.Sprintf("Default|%s", npConfig.Default),
		}
		if len(npConfig.Allowed) > 0 {
			npConfigOut = append(npConfigOut, fmt.Sprintf("Allowed|%s", strings.Join(npConfig.Allowed, ", ")))
		}
		if len(npConfig.Denied) > 0 {
			npConfigOut = append(npConfigOut, fmt.Sprintf("Denied|%s", strings.Join(npConfig.Denied, ", ")))
		}
		c.Ui.Output(formatKV(npConfigOut))
	}

	return 0
}

// formatNamespaceBasics formats the basic information of the namespace
func formatNamespaceBasics(ns *api.Namespace) string {
	enabled_drivers := "*"
	disabled_drivers := ""
	if ns.Capabilities != nil {
		if len(ns.Capabilities.EnabledTaskDrivers) != 0 {
			enabled_drivers = strings.Join(ns.Capabilities.EnabledTaskDrivers, ",")
		}
		if len(ns.Capabilities.DisabledTaskDrivers) != 0 {
			disabled_drivers = strings.Join(ns.Capabilities.DisabledTaskDrivers, ",")
		}
	}
	basic := []string{
		fmt.Sprintf("Name|%s", ns.Name),
		fmt.Sprintf("Description|%s", ns.Description),
		fmt.Sprintf("Quota|%s", ns.Quota),
		fmt.Sprintf("EnabledDrivers|%s", enabled_drivers),
		fmt.Sprintf("DisabledDrivers|%s", disabled_drivers),
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
		// search for an exact match in the returned namespaces
		for _, namespace := range namespaces {
			if namespace.Name == ns {
				return namespace, nil, nil
			}
		}
		// if not found, return the fuzzy matches.
		return nil, namespaces, nil
	}
}
