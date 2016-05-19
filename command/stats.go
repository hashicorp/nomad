package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/hashicorp/nomad/api"
)

type StatsCommand struct {
	Meta
}

func (f *StatsCommand) Help() string {
	return "Dispalys stats of an allocation or a task running on a nomad client"
}

func (f *StatsCommand) Synopsis() string {
	return "Dispalys stats of an allocation or a task running on a nomad client"
}

func (f *StatsCommand) Run(args []string) int {
	var verbose bool
	flags := f.Meta.FlagSet("fs-list", FlagSetClient)
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.Usage = func() { f.Ui.Output(f.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) < 1 {
		f.Ui.Error("allocation id is a required parameter")
		return 1
	}
	client, err := f.Meta.Client()
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	var allocID, task string
	allocID = strings.TrimSpace(args[0])
	if len(args) == 2 {
		task = strings.TrimSpace(args[1])
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(allocID) == 1 {
		f.Ui.Error(fmt.Sprintf("Alloc ID must contain at least two characters."))
		return 1
	}
	if len(allocID)%2 == 1 {
		// Identifiers must be of even length, so we strip off the last byte
		// to provide a consistent user experience.
		allocID = allocID[:len(allocID)-1]
	}

	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocs) == 0 {
		f.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
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
		f.Ui.Output(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", formatList(out)))
		return 0
	}
	// Prefix lookup matched a single allocation
	alloc, _, err := client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	stats, err := client.Allocations().Stats(alloc, nil)
	if err != nil {
		f.Ui.Error(fmt.Sprintf("unable to get stats: %v", err))
		return 1
	}
	if task == "" {
		f.printAllocResourceUsage(alloc, stats)
	} else {
		f.printTaskResourceUsage(task, stats)
	}
	return 0
}

func (f *StatsCommand) printTaskResourceUsage(task string, resourceUsage map[string]*api.TaskResourceUsage) {
	tu, ok := resourceUsage[task]
	if !ok {
		return
	}
	f.Ui.Output(fmt.Sprintf("===> Task: %q", task))
	f.Ui.Output("Memory Stats")
	out := make([]string, 2)
	out[0] = "RSS|Cache|Swap|Max Usage|Kernel Usage|KernelMaxUsage"
	out[1] = fmt.Sprintf("%v|%v|%v|%v|%v|%v",
		humanize.Bytes(tu.MemoryStats.RSS),
		humanize.Bytes(tu.MemoryStats.Cache),
		humanize.Bytes(tu.MemoryStats.Swap),
		humanize.Bytes(tu.MemoryStats.MaxUsage),
		humanize.Bytes(tu.MemoryStats.KernelUsage),
		humanize.Bytes(tu.MemoryStats.KernelMaxUsage),
	)
	f.Ui.Output(formatList(out))

	f.Ui.Output("")

	f.Ui.Output("CPU Stats")
	out = make([]string, 2)
	out[0] = "Percent|Throttled Periods|Throttled Time"
	percent := strconv.FormatFloat(tu.CpuStats.Percent, 'f', 2, 64)
	out[1] = fmt.Sprintf("%v|%v|%v|%v", percent,
		tu.CpuStats.ThrottledPeriods, tu.CpuStats.ThrottledTime)
	f.Ui.Output(formatList(out))
}

func (f *StatsCommand) printAllocResourceUsage(alloc *api.Allocation, resourceUsage map[string]*api.TaskResourceUsage) {
	f.Ui.Output(fmt.Sprintf("Resource Usage of Tasks running in Allocation %q", alloc.ID))
	for task, _ := range alloc.TaskStates {
		f.printTaskResourceUsage(task, resourceUsage)
	}
}
