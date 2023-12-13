// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type NamespaceCommand struct {
	Meta
}

func (f *NamespaceCommand) Help() string {
	helpText := `
Usage: nomad namespace <subcommand> [options] [args]

  This command groups subcommands for interacting with namespaces. Namespaces
  allow jobs and their associated objects to be segmented from each other and
  other users of the cluster. For a full guide on namespaces see:
  https://learn.hashicorp.com/tutorials/nomad/namespaces

  Create or update a namespace:

      $ nomad namespace apply -description "My new namespace" <name> 

  List namespaces:

      $ nomad namespace list

  View the status of a namespace:

      $ nomad namespace status <name>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *NamespaceCommand) Synopsis() string {
	return "Interact with namespaces"
}

func (f *NamespaceCommand) Name() string { return "namespace" }

func (f *NamespaceCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// NamespacePredictor returns a namespace predictor that can optionally filter
// specific namespaces
func NamespacePredictor(factory ApiClientFactory, filter map[string]struct{}) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Namespaces, nil)
		if err != nil {
			return []string{}
		}

		// Filter the returned namespaces. We assign the unfiltered slice to the
		// filtered slice but with no elements. This causes the slices to share
		// the underlying array and makes the filtering allocation free.
		unfiltered := resp.Matches[contexts.Namespaces]
		filtered := unfiltered[:0]
		for _, ns := range unfiltered {
			if _, ok := filter[ns]; !ok {
				filtered = append(filtered, ns)
			}
		}

		return filtered
	})
}
