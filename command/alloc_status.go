// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocStatusCommand struct {
	Meta
}

func (c *AllocStatusCommand) Help() string {
	helpText := `
Usage: nomad alloc status [options] <allocation>

  Display information about existing allocations and its tasks. This command can
  be used to inspect the current status of an allocation, including its running
  status, metadata, and verbose failure messages reported by internal
  subsystems.

  When ACLs are enabled, this command requires a token with the 'read-job' and
  'list-jobs' capabilities for the allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

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

func (c *AllocStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-short":   complete.PredictNothing,
			"-verbose": complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *AllocStatusCommand) AutocompleteArgs() complete.Predictor {
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

func (c *AllocStatusCommand) Name() string { return "alloc status" }

func (c *AllocStatusCommand) Run(args []string) int {
	var short, displayStats, verbose, json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
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
	if len(args) == 0 && (json || len(tmpl) > 0) {
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
		c.Ui.Error("This command takes one of the following argument conditions:")
		c.Ui.Error(" * A single <allocation>")
		c.Ui.Error(" * No arguments, with output format specified")
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
		c.Ui.Error("Identifier must contain at least two characters.")
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)
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
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
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
	if short {
		c.Ui.Output(formatAllocShortInfo(alloc, client))
	} else {
		output, err := formatAllocBasicInfo(alloc, client, length, verbose)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		c.Ui.Output(output)

		// add allocation network addresses
		if alloc.AllocatedResources != nil && len(alloc.AllocatedResources.Shared.Networks) > 0 && alloc.AllocatedResources.Shared.Networks[0].HasPorts() {
			c.Ui.Output("")
			c.Ui.Output(formatAllocNetworkInfo(alloc))
		}

		// add allocation nomad service discovery checks
		if checkOutput := formatAllocNomadServiceChecks(alloc.ID, client); checkOutput != "" {
			c.Ui.Output("")
			c.Ui.Output(checkOutput)
		}
	}

	if short {
		c.shortTaskStatus(alloc)
	} else {
		var statsErr error
		var stats *api.AllocResourceUsage
		stats, statsErr = client.Allocations().Stats(alloc, nil)
		if statsErr != nil {
			c.Ui.Output("")
			if statsErr != api.NodeDownErr {
				c.Ui.Error(fmt.Sprintf("Couldn't retrieve stats: %v", statsErr))
			} else {
				c.Ui.Output("Omitting resource statistics since the node is down.")
			}
		}
		c.outputTaskDetails(alloc, stats, displayStats, verbose)
	}

	// Format the detailed status
	if verbose {
		c.Ui.Output(c.Colorize().Color("\n[bold]Placement Metrics[reset]"))
		c.Ui.Output(formatAllocMetrics(alloc.Metrics, true, "  "))
	}

	return 0
}

func formatAllocShortInfo(alloc *api.Allocation, client *api.Client) string {
	formattedCreateTime := prettyTimeDiff(time.Unix(0, alloc.CreateTime), time.Now())
	formattedModifyTime := prettyTimeDiff(time.Unix(0, alloc.ModifyTime), time.Now())

	basic := []string{
		fmt.Sprintf("ID|%s", alloc.ID),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("Created|%s", formattedCreateTime),
		fmt.Sprintf("Modified|%s", formattedModifyTime),
	}

	return formatKV(basic)
}

func formatAllocBasicInfo(alloc *api.Allocation, client *api.Client, uuidLength int, verbose bool) (string, error) {
	var formattedCreateTime, formattedModifyTime string

	if verbose {
		formattedCreateTime = formatUnixNanoTime(alloc.CreateTime)
		formattedModifyTime = formatUnixNanoTime(alloc.ModifyTime)
	} else {
		formattedCreateTime = prettyTimeDiff(time.Unix(0, alloc.CreateTime), time.Now())
		formattedModifyTime = prettyTimeDiff(time.Unix(0, alloc.ModifyTime), time.Now())
	}

	basic := []string{
		fmt.Sprintf("ID|%s", alloc.ID),
		fmt.Sprintf("Eval ID|%s", limit(alloc.EvalID, uuidLength)),
		fmt.Sprintf("Name|%s", alloc.Name),
		fmt.Sprintf("Node ID|%s", limit(alloc.NodeID, uuidLength)),
		fmt.Sprintf("Node Name|%s", alloc.NodeName),
		fmt.Sprintf("Job ID|%s", alloc.JobID),
		fmt.Sprintf("Job Version|%d", *alloc.Job.Version),
		fmt.Sprintf("Client Status|%s", alloc.ClientStatus),
		fmt.Sprintf("Client Description|%s", alloc.ClientDescription),
		fmt.Sprintf("Desired Status|%s", alloc.DesiredStatus),
		fmt.Sprintf("Desired Description|%s", alloc.DesiredDescription),
		fmt.Sprintf("Created|%s", formattedCreateTime),
		fmt.Sprintf("Modified|%s", formattedModifyTime),
	}

	if alloc.DeploymentID != "" {
		health := "unset"
		canary := false
		if alloc.DeploymentStatus != nil {
			if alloc.DeploymentStatus.Healthy != nil {
				if *alloc.DeploymentStatus.Healthy {
					health = "healthy"
				} else {
					health = "unhealthy"
				}
			}

			canary = alloc.DeploymentStatus.Canary
		}

		basic = append(basic,
			fmt.Sprintf("Deployment ID|%s", limit(alloc.DeploymentID, uuidLength)),
			fmt.Sprintf("Deployment Health|%s", health))
		if canary {
			basic = append(basic, fmt.Sprintf("Canary|%v", true))
		}
	}

	if alloc.RescheduleTracker != nil && len(alloc.RescheduleTracker.Events) > 0 {
		attempts, total := alloc.RescheduleInfo(time.Unix(0, alloc.ModifyTime))
		// Show this section only if the reschedule policy limits the number of attempts
		if total > 0 {
			reschedInfo := fmt.Sprintf("Reschedule Attempts|%d/%d", attempts, total)
			basic = append(basic, reschedInfo)
		}
	}
	if alloc.NextAllocation != "" {
		basic = append(basic,
			fmt.Sprintf("Replacement Alloc ID|%s", limit(alloc.NextAllocation, uuidLength)))
	}
	if alloc.FollowupEvalID != "" {
		nextEvalTime := futureEvalTimePretty(alloc.FollowupEvalID, client)
		if nextEvalTime != "" {
			basic = append(basic,
				fmt.Sprintf("Reschedule Eligibility|%s", nextEvalTime))
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

func formatAllocNetworkInfo(alloc *api.Allocation) string {
	nw := alloc.AllocatedResources.Shared.Networks[0]
	addrs := []string{"Label|Dynamic|Address"}
	portFmt := func(label string, value, to int, hostIP, dyn string) string {
		s := fmt.Sprintf("%s|%s|%s:%d", label, dyn, hostIP, value)
		if to > 0 {
			s += fmt.Sprintf(" -> %d", to)
		}
		return s
	}
	if len(alloc.AllocatedResources.Shared.Ports) > 0 {
		for _, port := range alloc.AllocatedResources.Shared.Ports {
			addrs = append(addrs, portFmt("*"+port.Label, port.Value, port.To, port.HostIP, "yes"))
		}
	} else {
		for _, port := range nw.DynamicPorts {
			addrs = append(addrs, portFmt(port.Label, port.Value, port.To, nw.IP, "yes"))
		}
		for _, port := range nw.ReservedPorts {
			addrs = append(addrs, portFmt(port.Label, port.Value, port.To, nw.IP, "yes"))
		}
	}

	var mode string
	if nw.Mode != "" {
		mode = fmt.Sprintf(" (mode = %q)", nw.Mode)
	}

	return fmt.Sprintf("Allocation Addresses%s:\n%s", mode, formatList(addrs))
}

func formatAllocNomadServiceChecks(allocID string, client *api.Client) string {
	statuses, err := client.Allocations().Checks(allocID, nil)
	if err != nil {
		return ""
	} else if len(statuses) == 0 {
		return ""
	}
	results := []string{"Service|Task|Name|Mode|Status"}
	for _, status := range statuses {
		task := "(group)"
		if status.Task != "" {
			task = status.Task
		}
		// check | group | mode | status
		s := fmt.Sprintf("%s|%s|%s|%s|%s", status.Service, task, status.Check, status.Mode, status.Status)
		results = append(results, s)
	}
	sort.Strings(results[1:])
	return fmt.Sprintf("Nomad Service Checks:\n%s", formatList(results))
}

// futureEvalTimePretty returns when the eval is eligible to reschedule
// relative to current time, based on the WaitUntil field
func futureEvalTimePretty(evalID string, client *api.Client) string {
	evaluation, _, err := client.Evaluations().Info(evalID, nil)
	// Eval time is not a critical output,
	// don't return it on errors, if its not set or already in the past
	if err != nil || evaluation.WaitUntil.IsZero() || time.Now().After(evaluation.WaitUntil) {
		return ""
	}
	return prettyTimeDiff(evaluation.WaitUntil, time.Now())
}

// outputTaskDetails prints task details for each task in the allocation,
// optionally printing verbose statistics if displayStats is set
func (c *AllocStatusCommand) outputTaskDetails(alloc *api.Allocation, stats *api.AllocResourceUsage, displayStats bool, verbose bool) {
	taskLifecycles := map[string]*api.TaskLifecycle{}
	for _, t := range alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks {
		taskLifecycles[t.Name] = t.Lifecycle
	}

	for _, task := range c.sortedTaskStateIterator(alloc.TaskStates, taskLifecycles) {
		state := alloc.TaskStates[task]

		lcIndicator := ""
		if lc := taskLifecycles[task]; !lc.Empty() {
			lcIndicator = " (" + lifecycleDisplayName(lc) + ")"
		}

		c.Ui.Output(c.Colorize().Color(fmt.Sprintf("\n[bold]Task %q%v is %q[reset]", task, lcIndicator, state.State)))
		c.outputTaskResources(alloc, task, stats, displayStats)
		c.Ui.Output("")
		c.outputTaskVolumes(alloc, task, verbose)
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
		msg := event.DisplayMessage
		if msg == "" {
			msg = buildDisplayMessage(event)
		}
		formattedTime := formatUnixNanoTime(event.Time)
		events[size-i] = fmt.Sprintf("%s|%s|%s", formattedTime, event.Type, msg)
		// Reverse order so we are sorted by time
	}
	c.Ui.Output(formatList(events))
}

func buildDisplayMessage(event *api.TaskEvent) string {
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
		if event.RestartReason != "" && event.RestartReason != api.AllocRestartReasonWithinPolicy {
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
	case api.TaskClientReconnected:
		desc = "Client reconnected"
	default:
		desc = event.Message
	}

	return desc
}

// outputTaskResources prints the task resources for the passed task and if
// displayStats is set, verbose resource usage statistics
func (c *AllocStatusCommand) outputTaskResources(alloc *api.Allocation, task string, stats *api.AllocResourceUsage, displayStats bool) {
	resource, ok := alloc.TaskResources[task]
	if !ok {
		return
	}

	c.Ui.Output("Task Resources:")
	var addr []string
	for _, nw := range resource.Networks {
		ports := append(nw.DynamicPorts, nw.ReservedPorts...) //nolint:gocritic
		for _, port := range ports {
			addr = append(addr, fmt.Sprintf("%v: %v:%v\n", port.Label, nw.IP, port.Value))
		}
	}

	var resourcesOutput []string
	cpuHeader := "CPU"
	if resource.Cores != nil && *resource.Cores > 0 {
		cpuHeader = fmt.Sprintf("CPU (%v cores)", *resource.Cores)
	}
	resourcesOutput = append(resourcesOutput, fmt.Sprintf("%s|Memory|Disk|Addresses", cpuHeader))
	firstAddr := ""
	secondAddr := ""
	if len(addr) > 0 {
		firstAddr = addr[0]
	}
	if len(addr) > 1 {
		secondAddr = addr[1]
	}

	// Display the rolled up stats. If possible prefer the live statistics
	cpuUsage := strconv.Itoa(*resource.CPU)
	memUsage := humanize.IBytes(uint64(*resource.MemoryMB * bytesPerMegabyte))
	memMax := ""
	if max := resource.MemoryMaxMB; max != nil && *max != 0 && *max != *resource.MemoryMB {
		memMax = "Max: " + humanize.IBytes(uint64(*resource.MemoryMaxMB*bytesPerMegabyte))
	}
	var deviceStats []*api.DeviceGroupStats

	if stats != nil {
		if ru, ok := stats.Tasks[task]; ok && ru != nil && ru.ResourceUsage != nil {
			if cs := ru.ResourceUsage.CpuStats; cs != nil {
				cpuUsage = fmt.Sprintf("%v/%v", math.Floor(cs.TotalTicks), cpuUsage)
			}
			if ms := ru.ResourceUsage.MemoryStats; ms != nil {
				// Nomad uses RSS as the top-level metric to report, for historical reasons,
				// but it's not always measured (e.g. with cgroup-v2)
				usage := ms.RSS
				if usage == 0 && !slices.Contains(ms.Measured, "RSS") {
					usage = ms.Usage
				}
				memUsage = fmt.Sprintf("%v/%v", humanize.IBytes(usage), memUsage)
			}
			deviceStats = ru.ResourceUsage.DeviceStats
		}
	}
	resourcesOutput = append(resourcesOutput, fmt.Sprintf("%v MHz|%v|%v|%v",
		cpuUsage,
		memUsage,
		humanize.IBytes(uint64(*alloc.Resources.DiskMB*bytesPerMegabyte)),
		firstAddr))
	if memMax != "" || secondAddr != "" {
		resourcesOutput = append(resourcesOutput, fmt.Sprintf("|%v||%v", memMax, secondAddr))
	}
	for i := 2; i < len(addr); i++ {
		resourcesOutput = append(resourcesOutput, fmt.Sprintf("|||%v", addr[i]))
	}
	c.Ui.Output(formatListWithSpaces(resourcesOutput))

	if len(deviceStats) > 0 {
		c.Ui.Output("")
		c.Ui.Output("Device Stats")
		c.Ui.Output(formatList(getDeviceResources(deviceStats)))
	}

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
	deviceStats := resourceUsage.DeviceStats

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
			case "Usage":
				measuredStats = append(measuredStats, humanize.IBytes(memoryStats.Usage))
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

	if len(deviceStats) > 0 {
		c.Ui.Output("")
		c.Ui.Output("Device Stats")

		printDeviceStats(c.Ui, deviceStats)
	}
}

// shortTaskStatus prints out the current state of each task.
func (c *AllocStatusCommand) shortTaskStatus(alloc *api.Allocation) {
	tasks := make([]string, 0, len(alloc.TaskStates)+1)
	tasks = append(tasks, "Name|State|Last Event|Time|Lifecycle")

	taskLifecycles := map[string]*api.TaskLifecycle{}
	for _, t := range alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks {
		taskLifecycles[t.Name] = t.Lifecycle
	}

	for _, task := range c.sortedTaskStateIterator(alloc.TaskStates, taskLifecycles) {
		state := alloc.TaskStates[task]
		lastState := state.State
		var lastEvent, lastTime string

		l := len(state.Events)
		if l != 0 {
			last := state.Events[l-1]
			lastEvent = last.Type
			lastTime = formatUnixNanoTime(last.Time)
		}

		tasks = append(tasks, fmt.Sprintf("%s|%s|%s|%s|%s",
			task, lastState, lastEvent, lastTime, lifecycleDisplayName(taskLifecycles[task])))
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Tasks[reset]"))
	c.Ui.Output(formatList(tasks))
}

// sortedTaskStateIterator is a helper that takes the task state map and returns a
// channel that returns the keys in a sorted order.
func (c *AllocStatusCommand) sortedTaskStateIterator(m map[string]*api.TaskState, lifecycles map[string]*api.TaskLifecycle) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	// display prestart then prestart sidecar then main
	sort.SliceStable(keys, func(i, j int) bool {
		lci := lifecycles[keys[i]]
		lcj := lifecycles[keys[j]]

		switch {
		case lci == nil:
			return false
		case lcj == nil:
			return true
		case !lci.Sidecar && lcj.Sidecar:
			return true
		default:
			return false
		}
	})

	return keys
}

func lifecycleDisplayName(l *api.TaskLifecycle) string {
	if l.Empty() {
		return "main"
	}

	sidecar := ""
	if l.Sidecar {
		sidecar = " sidecar"
	}
	return l.Hook + sidecar
}

func (c *AllocStatusCommand) outputTaskVolumes(alloc *api.Allocation, taskName string, verbose bool) {
	var task *api.Task
	var tg *api.TaskGroup
FOUND:
	for _, tg = range alloc.Job.TaskGroups {
		for _, task = range tg.Tasks {
			if task.Name == taskName {
				break FOUND
			}
		}
	}
	if task == nil || tg == nil {
		c.Ui.Error(fmt.Sprintf("Could not find task data for %q", taskName))
		return
	}
	if len(task.VolumeMounts) == 0 {
		return
	}
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return
	}

	var hostVolumesOutput []string
	var csiVolumesOutput []string
	hostVolumesOutput = append(hostVolumesOutput, "ID|Read Only")
	if verbose {
		csiVolumesOutput = append(csiVolumesOutput,
			"Name|ID|Plugin|Provider|Schedulable|Read Only|Mount Options")
	} else {
		csiVolumesOutput = append(csiVolumesOutput, "ID|Read Only")
	}

	for _, volMount := range task.VolumeMounts {
		volReq := tg.Volumes[*volMount.Volume]
		switch volReq.Type {
		case api.CSIVolumeTypeHost:
			hostVolumesOutput = append(hostVolumesOutput,
				fmt.Sprintf("%s|%v", volReq.Name, *volMount.ReadOnly))
		case api.CSIVolumeTypeCSI:
			if verbose {
				source := volReq.Source
				if volReq.PerAlloc {
					source = source + api.AllocSuffix(alloc.Name)
				}

				// there's an extra API call per volume here so we toggle it
				// off with the -verbose flag
				vol, _, err := client.CSIVolumes().Info(source, nil)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error retrieving volume info for %q: %s",
						volReq.Name, err))
					continue
				}
				csiVolumesOutput = append(csiVolumesOutput,
					fmt.Sprintf("%s|%s|%s|%s|%v|%v|%s",
						volReq.Name,
						vol.ID,
						vol.PluginID,
						vol.Provider,
						vol.Schedulable,
						volReq.ReadOnly,
						csiVolMountOption(vol.MountOptions, volReq.MountOptions),
					))
			} else {
				csiVolumesOutput = append(csiVolumesOutput,
					fmt.Sprintf("%s|%v", volReq.Name, volReq.ReadOnly))
			}
		}
	}
	if len(hostVolumesOutput) > 1 {
		c.Ui.Output("Host Volumes:")
		c.Ui.Output(formatList(hostVolumesOutput))
		c.Ui.Output("") // line padding to next block
	}
	if len(csiVolumesOutput) > 1 {
		c.Ui.Output("CSI Volumes:")
		c.Ui.Output(formatList(csiVolumesOutput))
		c.Ui.Output("") // line padding to next block
	}
}
