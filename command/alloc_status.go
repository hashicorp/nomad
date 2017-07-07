package command

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/colorstring"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/client"
)

type AllocStatusCommand struct {
	Meta
	color *colorstring.Colorize
}

func (c *AllocStatusCommand) Help() string {
	helpText := `
Usage: nomad alloc-status [options] <allocation>

  Display information about existing allocations and its tasks. This command can
  be used to inspect the current status of an allocation, including its running
  status, metadata, and verbose failure messages reported by internal
  subsystems.

General Options:

  ` + generalOptionsUsage() + `

Alloc Status Options:

  -short
    Display short output. Shows only the most recent task event.

  -stats
    Display detailed resource usage statistics.

  -verbose
    Show full information.

  -json
    Output the allocation in its JSON format.

  -t
    Format and display allocation using a Go template.
`

	return strings.TrimSpace(helpText)
}

func (c *AllocStatusCommand) Synopsis() string {
	return "Display allocation status information and metadata"
}

func (c *AllocStatusCommand) Run(args []string) int {
	var short, displayStats, verbose, json bool
	var tmpl string

	flags := c.Meta.FlagSet("alloc-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&displayStats, "stats", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one allocation ID
	args = flags.Args()

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If args not specified but output format is specified, format and output the allocations data list
	if len(args) == 0 && json || len(tmpl) > 0 {
		allocs, _, err := client.Allocations().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying allocations: %v", err))
			return 1
		}

		out, err := Format(json, tmpl, allocs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	if len(args) != 1 {
		c.Ui.Error(c.Help())
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
		out := formatAllocListStubs(allocs, verbose, length)
		c.Ui.Output(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 0
	}
	// Prefix lookup matched a single allocation
	alloc, _, err := client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	// If output format is specified, format and output the data
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, alloc)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Format the allocation data
	output, err := formatAllocBasicInfo(alloc, client, length, verbose)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}
	c.Ui.Output(output)

	if short {
		c.shortTaskStatus(alloc)
	} else {
		var statsErr error
		var stats *api.AllocResourceUsage
		stats, statsErr = client.Allocations().Stats(alloc, nil)
		if statsErr != nil {
			c.Ui.Output("")
			if statsErr != api.NodeDownErr {
				c.Ui.Error(fmt.Sprintf("Couldn't retrieve stats (HINT: ensure Client.Advertise.HTTP is set): %v", statsErr))
			} else {
				c.Ui.Output("Omitting resource statistics since the node is down.")
			}
		}
		c.outputTaskDetails(alloc, stats, displayStats)
	}

	// Format the detailed status
	if verbose {
		c.Ui.Output(c.Colorize().Color("\n[bold]Placement Metrics[reset]"))
		c.Ui.Output(formatAllocMetrics(alloc.Metrics, true, "  "))
	}

	return 0
}

func formatAllocBasicInfo(alloc *api.Allocation, client *api.Client, uuidLength int, verbose bool) (string, error) {
	basic := []string{
		fmt.Sprintf("ID|%s", limit(alloc.ID, uuidLength)),
		fmt.Sprintf("Eval ID|%s", limit(alloc.EvalID, uuidLength)),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("Node ID|%s", limit(alloc.NodeID, uuidLength)),
		fmt.Sprintf("Job ID|%s", alloc.JobID),
		fmt.Sprintf("Job Version|%d", *alloc.Job.Version),
		fmt.Sprintf("Client Status|%s", alloc.ClientStatus),
		fmt.Sprintf("Client Description|%s", alloc.ClientDescription),
		fmt.Sprintf("Desired Status|%s", alloc.DesiredStatus),
		fmt.Sprintf("Desired Description|%s", alloc.DesiredDescription),
		fmt.Sprintf("Created At|%s", formatUnixNanoTime(alloc.CreateTime)),
	}

	if alloc.DeploymentID != "" {
		health := "unset"
		if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil {
			if *alloc.DeploymentStatus.Healthy {
				health = "healthy"
			} else {
				health = "unhealthy"
			}
		}

		basic = append(basic,
			fmt.Sprintf("Deployment ID|%s", limit(alloc.DeploymentID, uuidLength)),
			fmt.Sprintf("Deployment Health|%s", health))

		// Check if this allocation is a canary
		deployment, _, err := client.Deployments().Info(alloc.DeploymentID, nil)
		if err != nil {
			return "", fmt.Errorf("Error querying deployment %q: %s", alloc.DeploymentID, err)
		}

		canary := false
		if state, ok := deployment.TaskGroups[alloc.TaskGroup]; ok {
			for _, id := range state.PlacedCanaries {
				if id == alloc.ID {
					canary = true
					break
				}
			}
		}

		if canary {
			basic = append(basic, fmt.Sprintf("Canary|%v", true))
		}
	}

	if verbose {
		basic = append(basic,
			fmt.Sprintf("Evaluated Nodes|%d", alloc.Metrics.NodesEvaluated),
			fmt.Sprintf("Filtered Nodes|%d", alloc.Metrics.NodesFiltered),
			fmt.Sprintf("Exhausted Nodes|%d", alloc.Metrics.NodesExhausted),
			fmt.Sprintf("Allocation Time|%s", alloc.Metrics.AllocationTime),
			fmt.Sprintf("Failures|%d", alloc.Metrics.CoalescedFailures))
	}

	return formatKV(basic), nil
}

// outputTaskDetails prints task details for each task in the allocation,
// optionally printing verbose statistics if displayStats is set
func (c *AllocStatusCommand) outputTaskDetails(alloc *api.Allocation, stats *api.AllocResourceUsage, displayStats bool) {
	for task := range c.sortedTaskStateIterator(alloc.TaskStates) {
		state := alloc.TaskStates[task]
		c.Ui.Output(c.Colorize().Color(fmt.Sprintf("\n[bold]Task %q is %q[reset]", task, state.State)))
		c.outputTaskResources(alloc, task, stats, displayStats)
		c.Ui.Output("")
		c.outputTaskStatus(state)
	}
}

func formatTaskTimes(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	return formatTime(t)
}

// outputTaskStatus prints out a list of the most recent events for the given
// task state.
func (c *AllocStatusCommand) outputTaskStatus(state *api.TaskState) {
	basic := []string{
		fmt.Sprintf("Started At|%s", formatTaskTimes(state.StartedAt)),
		fmt.Sprintf("Finished At|%s", formatTaskTimes(state.FinishedAt)),
		fmt.Sprintf("Total Restarts|%d", state.Restarts),
		fmt.Sprintf("Last Restart|%s", formatTaskTimes(state.LastRestart))}

	c.Ui.Output("Task Events:")
	c.Ui.Output(formatKV(basic))
	c.Ui.Output("")

	c.Ui.Output("Recent Events:")
	events := make([]string, len(state.Events)+1)
	events[0] = "Time|Type|Description"

	size := len(state.Events)
	for i, event := range state.Events {
		formatedTime := formatUnixNanoTime(event.Time)

		// Build up the description based on the event type.
		var desc string
		switch event.Type {
		case api.TaskSetup:
			desc = event.Message
		case api.TaskStarted:
			desc = "Task started by client"
		case api.TaskReceived:
			desc = "Task received by client"
		case api.TaskFailedValidation:
			if event.ValidationError != "" {
				desc = event.ValidationError
			} else {
				desc = "Validation of task failed"
			}
		case api.TaskSetupFailure:
			if event.SetupError != "" {
				desc = event.SetupError
			} else {
				desc = "Task setup failed"
			}
		case api.TaskDriverFailure:
			if event.DriverError != "" {
				desc = event.DriverError
			} else {
				desc = "Failed to start task"
			}
		case api.TaskDownloadingArtifacts:
			desc = "Client is downloading artifacts"
		case api.TaskArtifactDownloadFailed:
			if event.DownloadError != "" {
				desc = event.DownloadError
			} else {
				desc = "Failed to download artifacts"
			}
		case api.TaskKilling:
			if event.KillReason != "" {
				desc = fmt.Sprintf("Killing task: %v", event.KillReason)
			} else if event.KillTimeout != 0 {
				desc = fmt.Sprintf("Sent interrupt. Waiting %v before force killing", event.KillTimeout)
			} else {
				desc = "Sent interrupt"
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
			in := fmt.Sprintf("Task restarting in %v", time.Duration(event.StartDelay))
			if event.RestartReason != "" && event.RestartReason != client.ReasonWithinPolicy {
				desc = fmt.Sprintf("%s - %s", event.RestartReason, in)
			} else {
				desc = in
			}
		case api.TaskNotRestarting:
			if event.RestartReason != "" {
				desc = event.RestartReason
			} else {
				desc = "Task exceeded restart policy"
			}
		case api.TaskSiblingFailed:
			if event.FailedSibling != "" {
				desc = fmt.Sprintf("Task's sibling %q failed", event.FailedSibling)
			} else {
				desc = "Task's sibling failed"
			}
		case api.TaskSignaling:
			sig := event.TaskSignal
			reason := event.TaskSignalReason

			if sig == "" && reason == "" {
				desc = "Task being sent a signal"
			} else if sig == "" {
				desc = reason
			} else if reason == "" {
				desc = fmt.Sprintf("Task being sent signal %v", sig)
			} else {
				desc = fmt.Sprintf("Task being sent signal %v: %v", sig, reason)
			}
		case api.TaskRestartSignal:
			if event.RestartReason != "" {
				desc = event.RestartReason
			} else {
				desc = "Task signaled to restart"
			}
		case api.TaskDriverMessage:
			desc = event.DriverMessage
		case api.TaskLeaderDead:
			desc = "Leader Task in Group dead"
		}

		// Reverse order so we are sorted by time
		events[size-i] = fmt.Sprintf("%s|%s|%s", formatedTime, event.Type, desc)
	}
	c.Ui.Output(formatList(events))
}

// outputTaskResources prints the task resources for the passed task and if
// displayStats is set, verbose resource usage statistics
func (c *AllocStatusCommand) outputTaskResources(alloc *api.Allocation, task string, stats *api.AllocResourceUsage, displayStats bool) {
	resource, ok := alloc.TaskResources[task]
	if !ok {
		return
	}

	c.Ui.Output("Task Resources")
	var addr []string
	for _, nw := range resource.Networks {
		ports := append(nw.DynamicPorts, nw.ReservedPorts...)
		for _, port := range ports {
			addr = append(addr, fmt.Sprintf("%v: %v:%v\n", port.Label, nw.IP, port.Value))
		}
	}
	var resourcesOutput []string
	resourcesOutput = append(resourcesOutput, "CPU|Memory|Disk|IOPS|Addresses")
	firstAddr := ""
	if len(addr) > 0 {
		firstAddr = addr[0]
	}

	// Display the rolled up stats. If possible prefer the live statistics
	cpuUsage := strconv.Itoa(*resource.CPU)
	memUsage := humanize.IBytes(uint64(*resource.MemoryMB * bytesPerMegabyte))
	if stats != nil {
		if ru, ok := stats.Tasks[task]; ok && ru != nil && ru.ResourceUsage != nil {
			if cs := ru.ResourceUsage.CpuStats; cs != nil {
				cpuUsage = fmt.Sprintf("%v/%v", math.Floor(cs.TotalTicks), cpuUsage)
			}
			if ms := ru.ResourceUsage.MemoryStats; ms != nil {
				memUsage = fmt.Sprintf("%v/%v", humanize.IBytes(ms.RSS), memUsage)
			}
		}
	}
	resourcesOutput = append(resourcesOutput, fmt.Sprintf("%v MHz|%v|%v|%v|%v",
		cpuUsage,
		memUsage,
		humanize.IBytes(uint64(*alloc.Resources.DiskMB*bytesPerMegabyte)),
		*resource.IOPS,
		firstAddr))
	for i := 1; i < len(addr); i++ {
		resourcesOutput = append(resourcesOutput, fmt.Sprintf("||||%v", addr[i]))
	}
	c.Ui.Output(formatListWithSpaces(resourcesOutput))

	if stats != nil {
		if ru, ok := stats.Tasks[task]; ok && ru != nil && displayStats && ru.ResourceUsage != nil {
			c.Ui.Output("")
			c.outputVerboseResourceUsage(task, ru.ResourceUsage)
		}
	}
}

// outputVerboseResourceUsage outputs the verbose resource usage for the passed
// task
func (c *AllocStatusCommand) outputVerboseResourceUsage(task string, resourceUsage *api.ResourceUsage) {
	memoryStats := resourceUsage.MemoryStats
	cpuStats := resourceUsage.CpuStats
	if memoryStats != nil && len(memoryStats.Measured) > 0 {
		c.Ui.Output("Memory Stats")

		// Sort the measured stats
		sort.Strings(memoryStats.Measured)

		var measuredStats []string
		for _, measured := range memoryStats.Measured {
			switch measured {
			case "RSS":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.RSS))
			case "Cache":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.Cache))
			case "Swap":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.Swap))
			case "Max Usage":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.MaxUsage))
			case "Kernel Usage":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.KernelUsage))
			case "Kernel Max Usage":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.KernelMaxUsage))
			}
		}

		out := make([]string, 2)
		out[0] = strings.Join(memoryStats.Measured, "|")
		out[1] = strings.Join(measuredStats, "|")
		c.Ui.Output(formatList(out))
		c.Ui.Output("")
	}

	if cpuStats != nil && len(cpuStats.Measured) > 0 {
		c.Ui.Output("CPU Stats")

		// Sort the measured stats
		sort.Strings(cpuStats.Measured)

		var measuredStats []string
		for _, measured := range cpuStats.Measured {
			switch measured {
			case "Percent":
				percent := strconv.FormatFloat(cpuStats.Percent, 'f', 2, 64)
				measuredStats = append(measuredStats, fmt.Sprintf("%v%%", percent))
			case "Throttled Periods":
				measuredStats = append(measuredStats, fmt.Sprintf("%v", cpuStats.ThrottledPeriods))
			case "Throttled Time":
				measuredStats = append(measuredStats, fmt.Sprintf("%v", cpuStats.ThrottledTime))
			case "User Mode":
				percent := strconv.FormatFloat(cpuStats.UserMode, 'f', 2, 64)
				measuredStats = append(measuredStats, fmt.Sprintf("%v%%", percent))
			case "System Mode":
				percent := strconv.FormatFloat(cpuStats.SystemMode, 'f', 2, 64)
				measuredStats = append(measuredStats, fmt.Sprintf("%v%%", percent))
			}
		}

		out := make([]string, 2)
		out[0] = strings.Join(cpuStats.Measured, "|")
		out[1] = strings.Join(measuredStats, "|")
		c.Ui.Output(formatList(out))
	}
}

// shortTaskStatus prints out the current state of each task.
func (c *AllocStatusCommand) shortTaskStatus(alloc *api.Allocation) {
	tasks := make([]string, 0, len(alloc.TaskStates)+1)
	tasks = append(tasks, "Name|State|Last Event|Time")
	for task := range c.sortedTaskStateIterator(alloc.TaskStates) {
		state := alloc.TaskStates[task]
		lastState := state.State
		var lastEvent, lastTime string

		l := len(state.Events)
		if l != 0 {
			last := state.Events[l-1]
			lastEvent = last.Type
			lastTime = formatUnixNanoTime(last.Time)
		}

		tasks = append(tasks, fmt.Sprintf("%s|%s|%s|%s",
			task, lastState, lastEvent, lastTime))
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Tasks[reset]"))
	c.Ui.Output(formatList(tasks))
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
