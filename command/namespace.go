package command

import (
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type NamespaceCommand struct {
	Meta
}

func (f *NamespaceCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *NamespaceCommand) Synopsis() string {
	return "Interact with namespaces"
}

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
