package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
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

  List evaluations:

      $ nomad eval list

  Examine an evaluations status:

      $ nomad eval status <eval-id>

  Delete evaluations:

      $ nomad eval delete <eval-id>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *EvalCommand) Synopsis() string {
	return "Interact with evaluations"
}

func (f *EvalCommand) Name() string { return "eval" }

func (f *EvalCommand) Run(_ []string) int { return cli.RunResultHelp }

// outputEvalList is a helper which outputs an array of evaluations as a list
// to the UI with key information such as ID and status.
func outputEvalList(ui cli.Ui, evals []*api.Evaluation, length int) {

	out := make([]string, len(evals)+1)
	out[0] = "ID|Priority|Triggered By|Job ID|Status|Placement Failures"
	for i, eval := range evals {
		failures, _ := evalFailureStatus(eval)
		out[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s|%s",
			limit(eval.ID, length),
			eval.Priority,
			eval.TriggeredBy,
			eval.JobID,
			eval.Status,
			failures,
		)
	}
	ui.Output(formatList(out))
}
