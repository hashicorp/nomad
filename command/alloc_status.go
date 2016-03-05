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


  -short
    Display short output. Shows only the most recent task event.

  -verbose
    Show full information.
`

	return strings.TrimSpace(helpText)
}

func (c *AllocStatusCommand) Synopsis() string {
	return "Display allocation status information and metadata"
}

func (c *AllocStatusCommand) Run(args []string) int {
	var short, verbose bool

	flags := c.Meta.FlagSet("alloc-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")

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

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		if len(allocID) == 1 {
			c.Ui.Error(fmt.Sprintf("Identifier must contain at least two characters."))
			return 1
		}
		if len(allocID)%2 == 1 {
			// Identifiers must be of even length, so we strip off the last byte
			// to provide a consistent user experience.
			allocID = allocID[:len(allocID)-1]
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
			out := make([]string, len(allocs)+1)
			out[0] = "ID|Eval ID|Job ID|Task Group|Desired Status|Client Status"
			for i, alloc := range allocs {
				out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
					limit(alloc.ID, length),
					limit(alloc.EvalID, length),
					alloc.JobID,
					alloc.TaskGroup,
					alloc.DesiredStatus,
					alloc.ClientStatus,
				)
			}
			c.Ui.Output(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", formatList(out)))
			return 0
		}
		// Prefix lookup matched a single allocation
		alloc, _, err = client.Allocations().Info(allocs[0].ID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
			return 1
		}
	}

	// Format the allocation data
	basic := []string{
		fmt.Sprintf("ID|%s", limit(alloc.ID, length)),
		fmt.Sprintf("Eval ID|%s", limit(alloc.EvalID, length)),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("Node ID|%s", limit(alloc.NodeID, length)),
		fmt.Sprintf("Job ID|%s", alloc.JobID),
		fmt.Sprintf("Client Status|%s", alloc.ClientStatus),
		fmt.Sprintf("Evaluated Nodes|%d", alloc.Metrics.NodesEvaluated),
		fmt.Sprintf("Filtered Nodes|%d", alloc.Metrics.NodesFiltered),
		fmt.Sprintf("Exhausted Nodes|%d", alloc.Metrics.NodesExhausted),
		fmt.Sprintf("Allocation Time|%s", alloc.Metrics.AllocationTime),
		fmt.Sprintf("Failures|%d", alloc.Metrics.CoalescedFailures),
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
	dumpAllocStatus(c.Ui, alloc, length)

	if !short {
		c.Ui.Output("\n==> Resources")
		c.taskResources(alloc)
	}

	return 0
}

// shortTaskStatus prints out the current state of each task.
func (c *AllocStatusCommand) shortTaskStatus(alloc *api.Allocation) {
	tasks := make([]string, 0, len(alloc.TaskStates)+1)
	tasks = append(tasks, "Name|State|Last Event|Time")
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
			case api.TaskStarted:
				desc = "Task started by client"
			case api.TaskReceived:
				desc = "Task received by client"
			case api.TaskDriverFailure:
				if event.DriverError != "" {
					desc = event.DriverError
				} else {
					desc = "Failed to start task"
				}
			case api.TaskKilled:
				if event.KillError != "" {
					desc = event.KillError
				} else {
					desc = "Task successfully killed"
				}
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
			case api.TaskRestarting:
				desc = fmt.Sprintf("Task restarting in %v", time.Duration(event.StartDelay))
			case api.TaskNotRestarting:
				desc = "Task exceeded restart policy"
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
	return formatTime(t)
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

// taskResources prints out the tasks current resource usage
func (c *AllocStatusCommand) taskResources(alloc *api.Allocation) {
	resources := make([]string, 2)
	resources[0] = "CPU|Memory MB|Disk MB|IOPS"
	resources[1] = fmt.Sprintf("%v|%v|%v|%v",
		alloc.Resources.CPU,
		alloc.Resources.MemoryMB,
		alloc.Resources.DiskMB,
		alloc.Resources.IOPS)
	c.Ui.Output(formatList(resources))
}
