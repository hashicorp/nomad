package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SystemReconcileSummariesCommand struct {
	Meta
}

func (c *SystemReconcileSummariesCommand) Help() string {
	helpText := `
Usage: nomad system reconcile summaries [options]

  Reconciles the summaries of all registered jobs.

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *SystemReconcileSummariesCommand) Synopsis() string {
	return "Reconciles the summaries of all registered jobs"
}

func (c *SystemReconcileSummariesCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *SystemReconcileSummariesCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SystemReconcileSummariesCommand) Name() string { return "system reconcile summaries" }

func (c *SystemReconcileSummariesCommand) Run(args []string) int {

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if err := client.System().ReconcileSummaries(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error running system summary reconciliation: %s", err))
		return 1
	}
	return 0
}
