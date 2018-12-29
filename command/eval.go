package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type EvalCommand struct {
	Meta
}

func (f *EvalCommand) Help() string {
	helpText := `
Usage: nomad eval <subcommand> [options] [args]

  This command groups subcommands for interacting with evaluations. Evaluations
  are used to trigger a scheduling event. As such, evaluations are an internal
  detail but can be useful for debugging placement failures when the cluster
  does not have the resources to run a given job.

  Examine an evaluations status:

      $ nomad eval status <eval-id>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *EvalCommand) Synopsis() string {
	return "Interact with evaluations"
}

func (f *EvalCommand) Name() string { return "eval" }

func (f *EvalCommand) Run(args []string) int {
	return cli.RunResultHelp
}
