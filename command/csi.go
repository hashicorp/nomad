package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type CSICommand struct {
	Meta
}

func (f *CSICommand) Help() string {
	helpText := `
Usage: nomad csi [plugin|volume] <subcommand> [options] [args]

  This command groups subcommands for interacting with CSI (Container Storage
  Interface) Plugins and Volumes. CSI volumes provide network storage devices
  to jobs running in containers. Healthy running plugins are necessary to use
  volumes, plugins are configured in jobs running on the cluster. For a full
  guide see https://www.nomadproject.io/guides/csi.html

  Examine a plugin's status:

      $ nomad csi plugin status <id>

  List existing plugins:

      $ nomad csi plugin list

  Examine a volume's status:

      $ nomad csi volume status <id>

  List existing volumes:

      $ nomad csi volume list

  Create a new volume:

      $ nomad csi volume register <file>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *CSICommand) Synopsis() string {
	return "Interact with quotas"
}

func (f *CSICommand) Name() string { return "csi" }

func (f *CSICommand) Run(args []string) int {
	return cli.RunResultHelp
}

// // CSIPredictor returns a csi predictor
// func CSIPredictor(factory ApiClientFactory) complete.Predictor {
// 	return complete.PredictFunc(func(a complete.Args) []string {
// 		client, err := factory()
// 		if err != nil {
// 			return nil
// 		}

// 		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.CSIs, nil)
// 		if err != nil {
// 			return []string{}
// 		}
// 		return resp.Matches[contexts.CSIs]
// 	})
// }
