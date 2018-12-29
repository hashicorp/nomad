package command

import (
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type QuotaCommand struct {
	Meta
}

func (f *QuotaCommand) Help() string {
	return "This command is accessed by using one of the subcommands below."
}

func (f *QuotaCommand) Synopsis() string {
	return "Interact with quotas"
}

func (f *QuotaCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// QuotaPredictor returns a quota predictor
func QuotaPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Quotas, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Quotas]
	})
}
