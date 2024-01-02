// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure ServiceListCommand satisfies the cli.Command interface.
var _ cli.Command = &ServiceListCommand{}

// ServiceListCommand implements cli.Command.
type ServiceListCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ServiceListCommand) Help() string {
	helpText := `
Usage: nomad service list [options]

  List is used to list the currently registered services.

  If ACLs are enabled, this command requires a token with the 'read-job'
  capabilities for the namespace of all services. Any namespaces that the token
  does not have access to will have its services filtered from the results.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Service List Options:

  -json
    Output the services in JSON format.

  -t
    Format and display the services using a Go template.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ServiceListCommand) Synopsis() string {
	return "Display all registered Nomad services"
}

func (s *ServiceListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(s.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

// Name returns the name of this command.
func (s *ServiceListCommand) Name() string { return "service list" }

// Run satisfies the cli.Command Run function.
func (s *ServiceListCommand) Run(args []string) int {

	var (
		json       bool
		tmpl, name string
	)

	flags := s.Meta.FlagSet(s.Name(), FlagSetClient)
	flags.Usage = func() { s.Ui.Output(s.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&tmpl, "t", "", "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		s.Ui.Error("This command takes no arguments")
		s.Ui.Error(commandErrorText(s))
		return 1
	}

	client, err := s.Meta.Client()
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	list, _, err := client.Services().List(nil)
	if err != nil {
		s.Ui.Error(fmt.Sprintf("Error listing service registrations: %s", err))
		return 1
	}

	if len(list) == 0 {
		s.Ui.Output("No service registrations found")
		return 0
	}

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, list)
		if err != nil {
			s.Ui.Error(err.Error())
			return 1
		}
		s.Ui.Output(out)
		return 0
	}

	s.formatOutput(list)
	return 0
}

func (s *ServiceListCommand) formatOutput(regs []*api.ServiceRegistrationListStub) {

	// Create objects to hold sorted a sorted namespace array and a mapping, so
	// we can perform service lookups on a namespace basis.
	sortedNamespaces := make([]string, len(regs))
	namespacedServices := make(map[string][]*api.ServiceRegistrationStub)

	for i, namespaceServices := range regs {
		sortedNamespaces[i] = namespaceServices.Namespace
		namespacedServices[namespaceServices.Namespace] = namespaceServices.Services
	}

	// Sort the namespaces.
	sort.Strings(sortedNamespaces)

	// The table always starts with the service name.
	outputTable := []string{"Service Name"}

	// If the request was made using the wildcard namespace, include this in
	// the output.
	if s.Meta.namespace == api.AllNamespacesNamespace {
		outputTable[0] += "|Namespace"
	}

	// The tags come last and are always present.
	outputTable[0] += "|Tags"

	for _, ns := range sortedNamespaces {

		// Grab the services belonging to this namespace.
		services := namespacedServices[ns]

		// Create objects to hold sorted a sorted service name array and a
		// mapping, so we can perform service tag lookups on a name basis.
		sortedNames := make([]string, len(services))
		serviceTags := make(map[string][]string)

		for i, service := range services {
			sortedNames[i] = service.ServiceName
			serviceTags[service.ServiceName] = service.Tags
		}

		// Sort the service names.
		sort.Strings(sortedNames)

		for _, serviceName := range sortedNames {

			// Grab the service tags, and sort these for good measure.
			tags := serviceTags[serviceName]
			sort.Strings(tags)

			// Build the output array entry.
			regOutput := serviceName

			if s.Meta.namespace == api.AllNamespacesNamespace {
				regOutput += "|" + ns
			}
			regOutput += "|" + fmt.Sprintf("[%s]", strings.Join(tags, ","))
			outputTable = append(outputTable, regOutput)
		}
	}

	s.Ui.Output(formatList(outputTable))
}
