// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocPauseCommand struct {
	Meta
}

func (c *AllocPauseCommand) Help() string {
	helpText := `
Usage: nomad alloc pause [options] <allocation> <task>

  Set the pause state of an allocation. This command is used to suspend the
  operation of a specific task.

  When ACLs are enabled, this command requires the job-submit capability for
  the allocation's namespace.

General Options:

` + generalOptionsUsage(usageOptsDefault) + `

Pause Specific Options:

  -state=<state>
    Specify the schedule state to apply to a task. Must be one of pause, run,
	or scheduled. When set to pause the task is halted. When set to run the task
	is started regardless of the task schedule. When in scheduled state the task
	respects the task schedule state in the task configuration. Defaults to
	pause.

  -status
    Get the current task schedule state status.

  -task=<task-name>
    Specify the individual task that the action will apply to. If task name is
    given with both an argument and the '-task' option, preference is given to
    the '-task' option.

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocPauseCommand) Name() string { return "alloc pause" }

func (c *AllocPauseCommand) Run(args []string) int {
	var verbose, status bool
	var action, task string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&action, "state", "pause", "")
	flags.BoolVar(&status, "status", false, "")
	flags.StringVar(&task, "task", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one alloc
	args = flags.Args()
	if len(args) < 1 || len(args) > 2 {
		c.Ui.Error("This command takes up to two arguments: <alloc-id> <task>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	allocID := args[0]

	// Truncate the id unless full length is required
	length := shortId
	if verbose {
		length = fullId
	}

	// Ensure the specified action is valid
	actions := []string{"pause", "run", "scheduled"}
	if !slices.Contains(actions, action) {
		c.Ui.Error(fmt.Sprintf("Pause action must be one of %q, %q, or %q but got %q",
			"pause", "run", "scheduled", action,
		))
		return 1
	}

	// Query the allocation info
	if len(allocID) == 1 {
		c.Ui.Error("Alloc ID must contain at least two characters.")
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)

	// Get the HTTP Client
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
		out := formatAllocListStubs(allocs, verbose, length)
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}

	// Prefix lookup matched a single allocation, yay
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// If -task is not provided then fallback to reading the task name from args
	if task == "" && len(args) >= 2 {
		task = args[1]
	}

	// Ensure the task (if specified) exists in the allocation
	if task != "" {
		err = validateTaskExistsInAllocation(task, alloc)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	// If this is a -status request, fetch & print the status, then exit
	if status {
		query := &api.QueryOptions{
			Params: map[string]string{"task": task},
		}
		state, _, err := client.Allocations().GetPauseState(alloc, query, task)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting task pause state: %s", err))
			return 1
		}
		c.Ui.Info(fmt.Sprintf("Pause state: %q", state))
		return 0
	}

	// Send the pause state
	err = client.Allocations().SetPauseState(alloc, nil, task, action)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting pause state: %s", err))
		return 1
	}

	return 0
}

func (c *AllocPauseCommand) Synopsis() string {
	return "Pause a running task"
}

func (c *AllocPauseCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-mode":    complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		},
	)
}

func (c *AllocPauseCommand) AutocompleteArgs() complete.Predictor {
	// Here we only autocomplete allocation names.
	// In a similar comment we remark about autocompleting tasks one day.
	// Wouldn't that be nice?
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
