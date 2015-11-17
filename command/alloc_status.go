package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
)

type AllocStatusCommand struct {
	Meta
}

func (c *AllocStatusCommand) Help() string {
	helpText := `
Usage: nomad alloc-status [options] <allocation>

  Display information about existing allocations and its tasks. This command can
  be used to inspect the current status of all allocation, including its running
  status, metadata, and verbose failure messages reported by internal
  subsystems.

General Options:

  ` + generalOptionsUsage() + `

Alloc Status Options:

  -short
    Display short output. Shows only the most recent task event.
`

	return strings.TrimSpace(helpText)
}

func (c *AllocStatusCommand) Synopsis() string {
	return "Display allocation status information and metadata"
}

func (c *AllocStatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet("alloc-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one allocation ID
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	allocID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Query the allocation info
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// Format the allocation data
	basic := []string{
		fmt.Sprintf("ID|%s", alloc.ID),
		fmt.Sprintf("EvalID|%s", alloc.EvalID),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("NodeID|%s", alloc.NodeID),
		fmt.Sprintf("JobID|%s", alloc.JobID),
		fmt.Sprintf("ClientStatus|%s", alloc.ClientStatus),
		fmt.Sprintf("NodesEvaluated|%d", alloc.Metrics.NodesEvaluated),
		fmt.Sprintf("NodesFiltered|%d", alloc.Metrics.NodesFiltered),
		fmt.Sprintf("NodesExhausted|%d", alloc.Metrics.NodesExhausted),
		fmt.Sprintf("AllocationTime|%s", alloc.Metrics.AllocationTime),
		fmt.Sprintf("CoalescedFailures|%d", alloc.Metrics.CoalescedFailures),
	}
	c.Ui.Output(formatKV(basic))

	// Print the state of each task.
	if short {
		c.shortTaskStatus(alloc)
	} else {
		c.taskStatus(alloc)
	}

	// Format the detailed status
	c.Ui.Output("\n==> Status")
	dumpAllocStatus(c.Ui, alloc)

	return 0
}

// shortTaskStatus prints out the current state of each task.
func (c *AllocStatusCommand) shortTaskStatus(alloc *api.Allocation) {
	tasks := make([]string, 0, len(alloc.TaskStates)+1)
	tasks = append(tasks, "Name|State|LastEvent|Time")
	for task := range c.sortedTaskStateIterator(alloc.TaskStates) {
		fmt.Println(task)
		state := alloc.TaskStates[task]
		lastState := state.State
		var lastEvent, lastTime string

		l := len(state.Events)
		if l != 0 {
			last := state.Events[l-1]
			lastEvent = last.Type
			lastTime = c.formatUnixNanoTime(last.Time)
		}

		tasks = append(tasks, fmt.Sprintf("%s|%s|%s|%s",
			task, lastState, lastEvent, lastTime))
	}

	c.Ui.Output("\n==> Tasks")
	c.Ui.Output(formatList(tasks))
}

// taskStatus prints out the most recent events for each task.
func (c *AllocStatusCommand) taskStatus(alloc *api.Allocation) {
	for task := range c.sortedTaskStateIterator(alloc.TaskStates) {
		state := alloc.TaskStates[task]
		events := make([]string, len(state.Events)+1)
		events[0] = "Time|Type|Description"

		size := len(state.Events)
		for i, event := range state.Events {
			formatedTime := c.formatUnixNanoTime(event.Time)

			// Build up the description based on the event type.
			var desc string
			switch event.Type {
			case api.TaskDriverFailure:
				desc = event.DriverError
			case api.TaskKilled:
				desc = event.KillError
			case api.TaskTerminated:
				var parts []string
				parts = append(parts, fmt.Sprintf("Exit Code: %d", event.ExitCode))

				if event.Signal != 0 {
					parts = append(parts, fmt.Sprintf("Signal: %d", event.Signal))
				}

				if event.Message != "" {
					parts = append(parts, fmt.Sprintf("Exit Message: %q", event.Message))
				}
				desc = strings.Join(parts, ", ")
			}

			// Reverse order so we are sorted by time
			events[size-i] = fmt.Sprintf("%s|%s|%s", formatedTime, event.Type, desc)
		}

		c.Ui.Output(fmt.Sprintf("\n==> Task %q is %q\nRecent Events:", task, state.State))
		c.Ui.Output(formatList(events))
	}
}

// formatUnixNanoTime is a helper for formating time for output.
func (c *AllocStatusCommand) formatUnixNanoTime(nano int64) string {
	t := time.Unix(0, nano)
	return t.Format("15:04:05 01/02/06")
}

// sortedTaskStateIterator is a helper that takes the task state map and returns a
// channel that returns the keys in a sorted order.
func (c *AllocStatusCommand) sortedTaskStateIterator(m map[string]*api.TaskState) <-chan string {
	output := make(chan string, len(m))
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	for _, key := range keys {
		output <- key
	}

	close(output)
	return output
}
