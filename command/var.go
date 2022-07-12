package command

import (
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

type VarCommand struct {
	Meta
}

func (f *VarCommand) Help() string {
	helpText := `
Usage: nomad var <subcommand> [options] [args]

  This command groups subcommands for interacting with secure variables. Secure
  variables allow operators to provide credentials and otherwise sensitive
  material to Nomad jobs at runtime via the template stanza or directly through
  the Nomad API and CLI.

  Users can create new secure variables; list, inspect, and delete existing
  secure variables, and more. For a full guide on secure variables see:
  https://www.nomadproject.io/guides/vars.html

  Create a secure variable specification file:

      $ nomad var init

  Upsert a secure variable:

      $ nomad var put <path>

  Examine a secure variable:

      $ nomad var get <path>

  List existing secure variables:

      $ nomad var list <prefix>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *VarCommand) Synopsis() string {
	return "Interact with secure variables"
}

func (f *VarCommand) Name() string { return "var" }

func (f *VarCommand) Run(args []string) int {
	return cli.RunResultHelp
}

// SecureVariablePathPredictor returns a var predictor
func SecureVariablePathPredictor(factory ApiClientFactory) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := factory()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.SecureVariables, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.SecureVariables]
	})
}
