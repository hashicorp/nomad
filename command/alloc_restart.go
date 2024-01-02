// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

func (c *AllocRestartCommand) Help() string {
	helpText := `
Usage: nomad alloc restart [options] <allocation> <task>

  Restart an existing allocation. This command is used to restart a specific alloc
  and its tasks. If no task is provided then all of the allocation's tasks that
  are currently running will be restarted.

  Use the option '-all-tasks' to restart tasks that have already run, such as
  non-sidecar prestart and poststart tasks.

  When ACLs are enabled, this command requires a token with the
  'alloc-lifecycle', 'read-job', and 'list-jobs' capabilities for the
  allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Restart Specific Options:

  -all-tasks
    If set, all tasks in the allocation will be restarted, even the ones that
    already ran. This option cannot be used with '-task' or the '<task>'
    argument.

  -task <task-name>
    Specify the individual task to restart. If task name is given with both an
    argument and the '-task' option, preference is given to the '-task' option.
    This option cannot be used with '-all-tasks'.

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocRestartCommand) Name() string { return "alloc restart" }

func (c *AllocRestartCommand) Run(args []string) int {
	var allTasks, verbose bool
	var task string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&allTasks, "all-tasks", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&task, "task", "", "")

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

	// If -task isn't provided fallback to reading the task name
	// from args.
	if task == "" && len(args) >= 2 {
		task = args[1]
	}

	if allTasks && task != "" {
		c.Ui.Error("The -all-tasks option is not allowed when restarting a specific task.")
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(allocID) == 1 {
		c.Ui.Error("Alloc ID must contain at least two characters.")
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

	if task != "" {
		err := validateTaskExistsInAllocation(task, alloc)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	if allTasks {
		err = client.Allocations().RestartAllTasks(alloc, nil)
	} else {
		err = client.Allocations().Restart(alloc, task, nil)
	}
	if err != nil {
		target := "allocation"
		if task != "" {
			target = "task"
		}
		c.Ui.Error(fmt.Sprintf("Failed to restart %s:\n\n%s", target, err.Error()))
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

func (c *AllocRestartCommand) Synopsis() string {
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
