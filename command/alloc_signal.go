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

type AllocSignalCommand struct {
	Meta
}

func (c *AllocSignalCommand) Help() string {
	helpText := `
Usage: nomad alloc signal [options] <allocation> <task>

  Signal an existing allocation. This command is used to signal a specific alloc
  and its subtasks. If no task is provided then all of the allocations subtasks
  will receive the signal.

  When ACLs are enabled, this command requires a token with the
  'alloc-lifecycle', 'read-job', and 'list-jobs' capabilities for the
  allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Signal Specific Options:

  -s
    Specify the signal that the selected tasks should receive. Defaults to SIGKILL.

  -task <task-name>
	Specify the individual task that will receive the signal. If task name is given
	with both an argument and the '-task' option, preference is given to the '-task'
	option.

  -verbose
    Show full information.
`
	return strings.TrimSpace(helpText)
}

func (c *AllocSignalCommand) Name() string { return "alloc signal" }

func (c *AllocSignalCommand) Run(args []string) int {
	var verbose bool
	var signal, task string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.StringVar(&signal, "s", "SIGKILL", "")
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

	// If -task isn't provided fallback to reading the task name
	// from args.
	if task == "" && len(args) >= 2 {
		task = args[1]
	}

	if task != "" {
		err := validateTaskExistsInAllocation(task, alloc)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	err = client.Allocations().Signal(alloc, nil, task, signal)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error signalling allocation: %s", err))
		return 1
	}

	return 0
}

func (c *AllocSignalCommand) Synopsis() string {
	return "Signal a running allocation"
}

func (c *AllocSignalCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-s":       complete.PredictNothing,
			"-verbose": complete.PredictNothing,
		})
}
func (c *AllocSignalCommand) AutocompleteArgs() complete.Predictor {
	// Here we only autocomplete allocation names. Eventually we may consider
	// expanding this to also autocomplete task names. To do so, we'll need to
	// either change the autocompletion api, or implement parsing such that we can
	// easily compute the current arg position.
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
