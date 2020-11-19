package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocRestartCommand struct {
	Meta
}

func (a *AllocRestartCommand) Help() string {
	helpText := `
Usage: nomad alloc restart [options] <allocation> <task>

  restart an existing allocation. This command is used to restart a specific alloc
  and its tasks. If no task is provided then all of the allocation's tasks will
  be restarted.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Restart Specific Options:

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocRestartCommand) Name() string { return "alloc restart" }

func (c *AllocRestartCommand) Run(args []string) int {
	var verbose bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one alloc
	args = flags.Args()
	if len(args) < 1 || len(args) > 2 {
		c.Ui.Error("This command takes one or two arguments: <alloc-id> <task-name>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	allocID := args[0]

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(allocID) == 1 {
		c.Ui.Error(fmt.Sprintf("Alloc ID must contain at least two characters."))
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}

	if len(allocs) == 0 {
		c.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}

	if len(allocs) > 1 {
		// Format the allocs
		out := formatAllocListStubs(allocs, verbose, length)
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}

	// Prefix lookup matched a single allocation
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	var taskName string
	if len(args) == 2 {
		// Validate Task
		taskName = args[1]
		err := validateTaskExistsInAllocation(taskName, alloc)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	err = client.Allocations().Restart(alloc, taskName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to restart allocation:\n\n%s", err.Error()))
		return 1
	}

	return 0
}

func validateTaskExistsInAllocation(taskName string, alloc *api.Allocation) error {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return fmt.Errorf("Could not find allocation task group: %s", alloc.TaskGroup)
	}

	foundTaskNames := make([]string, len(tg.Tasks))
	for i, task := range tg.Tasks {
		foundTaskNames[i] = task.Name
		if task.Name == taskName {
			return nil
		}
	}

	return fmt.Errorf("Could not find task named: %s, found:\n%s", taskName, formatList(foundTaskNames))
}

func (a *AllocRestartCommand) Synopsis() string {
	return "Restart a running allocation"
}

func (c *AllocRestartCommand) AutocompleteArgs() complete.Predictor {
	// Here we attempt to autocomplete allocations for any position of arg.
	// We should eventually try to auto complete the task name if the arg is
	// at position 2.
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Allocs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Allocs]
	})
}
