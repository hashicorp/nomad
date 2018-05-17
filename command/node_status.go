package command

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/posener/complete"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper"
)

const (
	// floatFormat is a format string for formatting floats.
	floatFormat = "#,###.##"

	// bytesPerMegabyte is the number of bytes per MB
	bytesPerMegabyte = 1024 * 1024
)

type NodeStatusCommand struct {
	Meta
	length      int
	short       bool
	verbose     bool
	list_allocs bool
	self        bool
	stats       bool
	json        bool
	tmpl        string
}

func (c *NodeStatusCommand) Help() string {
	helpText := `
Usage: nomad node status [options] <node>

  Display status information about a given node. The list of nodes
  returned includes only nodes which jobs may be scheduled to, and
  includes status and other high-level information.

  If a node ID is passed, information for that specific node will be displayed,
  including resource usage statistics. If no node ID's are passed, then a
  short-hand list of all nodes will be displayed. The -self flag is useful to
  quickly access the status of the local node.

General Options:

  ` + generalOptionsUsage() + `

Node Status Options:

  -self
    Query the status of the local node.

  -stats
    Display detailed resource usage statistics.

  -allocs
    Display a count of running allocations for each node.

  -short
    Display short output. Used only when a single node is being
    queried, and drops verbose output about node allocations.

  -verbose
    Display full information.

  -json
    Output the node in its JSON format.

  -t
    Format and display node using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeStatusCommand) Synopsis() string {
	return "Display status information about nodes"
}

func (c *NodeStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-allocs":  complete.PredictNothing,
			"-json":    complete.PredictNothing,
			"-self":    complete.PredictNothing,
			"-short":   complete.PredictNothing,
			"-stats":   complete.PredictNothing,
			"-t":       complete.PredictAnything,
			"-verbose": complete.PredictNothing,
		})
}

func (c *NodeStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Nodes, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Nodes]
	})
}

func (c *NodeStatusCommand) Name() string { return "node-status" }

func (c *NodeStatusCommand) Run(args []string) int {

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.short, "short", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")
	flags.BoolVar(&c.list_allocs, "allocs", false, "")
	flags.BoolVar(&c.self, "self", false, "")
	flags.BoolVar(&c.stats, "stats", false, "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either a single node or none
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either one or no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate the id unless full length is requested
	c.length = shortId
	if c.verbose {
		c.length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Use list mode if no node name was provided
	if len(args) == 0 && !c.self {

		// Query the node info
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node status: %s", err))
			return 1
		}

		// If output format is specified, format and output the node data list
		if c.json || len(c.tmpl) > 0 {
			out, err := Format(c.json, c.tmpl, nodes)
			if err != nil {
				c.Ui.Error(err.Error())
				return 1
			}

			c.Ui.Output(out)
			return 0
		}

		// Return nothing if no nodes found
		if len(nodes) == 0 {
			return 0
		}

		// Format the nodes list
		out := make([]string, len(nodes)+1)

		out[0] = "ID|DC|Name|Class|"

		if c.verbose {
			out[0] += "Address|Version|"
		}

		out[0] += "Drain|Eligibility|Status"

		if c.list_allocs {
			out[0] += "|Running Allocs"
		}

		for i, node := range nodes {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s",
				limit(node.ID, c.length),
				node.Datacenter,
				node.Name,
				node.NodeClass)
			if c.verbose {
				out[i+1] += fmt.Sprintf("|%s|%s",
					node.Address, node.Version)
			}
			out[i+1] += fmt.Sprintf("|%v|%s|%s",
				node.Drain,
				node.SchedulingEligibility,
				node.Status)

			if c.list_allocs {
				numAllocs, err := getRunningAllocs(client, node.ID)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error querying node allocations: %s", err))
					return 1
				}
				out[i+1] += fmt.Sprintf("|%v",
					len(numAllocs))
			}
		}

		// Dump the output
		c.Ui.Output(formatList(out))
		return 0
	}

	// Query the specific node
	var nodeID string
	if !c.self {
		nodeID = args[0]
	} else {
		var err error
		if nodeID, err = getLocalNodeID(client); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}
	if len(nodeID) == 1 {
		c.Ui.Error(fmt.Sprintf("Identifier must contain at least two characters."))
		return 1
	}

	nodeID = sanitizeUUIDPrefix(nodeID)
	nodes, _, err := client.Nodes().PrefixList(nodeID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
		return 1
	}
	// Return error if no nodes are found
	if len(nodes) == 0 {
		c.Ui.Error(fmt.Sprintf("No node(s) with prefix %q found", nodeID))
		return 1
	}
	if len(nodes) > 1 {
		// Dump the output
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple nodes\n\n%s",
			formatNodeStubList(nodes, c.verbose)))
		return 1
	}

	// Prefix lookup matched a single node
	node, _, err := client.Nodes().Info(nodes[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
		return 1
	}

	// If output format is specified, format and output the data
	if c.json || len(c.tmpl) > 0 {
		out, err := Format(c.json, c.tmpl, node)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	return c.formatNode(client, node)
}

func nodeDrivers(n *api.Node) []string {
	var drivers []string
	for k, v := range n.Attributes {
		// driver.docker = 1
		parts := strings.Split(k, ".")
		if len(parts) != 2 {
			continue
		} else if parts[0] != "driver" {
			continue
		} else if v != "1" {
			continue
		}

		drivers = append(drivers, parts[1])
	}

	sort.Strings(drivers)
	return drivers
}

func formatDrain(n *api.Node) string {
	if n.DrainStrategy != nil {
		b := new(strings.Builder)
		b.WriteString("true")

		if n.DrainStrategy.ForceDeadline.IsZero() {
			b.WriteString("; no deadline")
		} else {
			fmt.Fprintf(b, "; %s deadline", formatTime(n.DrainStrategy.ForceDeadline))
		}

		if n.DrainStrategy.IgnoreSystemJobs {
			b.WriteString("; ignoring system jobs")
		}
		return b.String()
	}

	return strconv.FormatBool(n.Drain)
}

func (c *NodeStatusCommand) formatNode(client *api.Client, node *api.Node) int {
	// Format the header output
	basic := []string{
		fmt.Sprintf("ID|%s", limit(node.ID, c.length)),
		fmt.Sprintf("Name|%s", node.Name),
		fmt.Sprintf("Class|%s", node.NodeClass),
		fmt.Sprintf("DC|%s", node.Datacenter),
		fmt.Sprintf("Drain|%v", formatDrain(node)),
		fmt.Sprintf("Eligibility|%s", node.SchedulingEligibility),
		fmt.Sprintf("Status|%s", node.Status),
	}

	if c.short {
		basic = append(basic, fmt.Sprintf("Drivers|%s", strings.Join(nodeDrivers(node), ",")))
		c.Ui.Output(c.Colorize().Color(formatKV(basic)))
	} else {
		// Get the host stats
		hostStats, nodeStatsErr := client.Nodes().Stats(node.ID, nil)
		if nodeStatsErr != nil {
			c.Ui.Output("")
			c.Ui.Error(fmt.Sprintf("error fetching node stats: %v", nodeStatsErr))
		}
		if hostStats != nil {
			uptime := time.Duration(hostStats.Uptime * uint64(time.Second))
			basic = append(basic, fmt.Sprintf("Uptime|%s", uptime.String()))
		}

		// Emit the driver info
		if !c.verbose {
			driverStatus := fmt.Sprintf("Driver Status| %s", c.outputTruncatedNodeDriverInfo(node))
			basic = append(basic, driverStatus)
		}

		c.Ui.Output(c.Colorize().Color(formatKV(basic)))

		if c.verbose {
			c.outputNodeDriverInfo(node)
		}

		// Emit node events
		c.outputNodeStatusEvents(node)

		// Get list of running allocations on the node
		runningAllocs, err := getRunningAllocs(client, node.ID)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node for running allocations: %s", err))
			return 1
		}

		allocatedResources := getAllocatedResources(client, runningAllocs, node)
		c.Ui.Output(c.Colorize().Color("\n[bold]Allocated Resources[reset]"))
		c.Ui.Output(formatList(allocatedResources))

		actualResources, err := getActualResources(client, runningAllocs, node)
		if err == nil {
			c.Ui.Output(c.Colorize().Color("\n[bold]Allocation Resource Utilization[reset]"))
			c.Ui.Output(formatList(actualResources))
		}

		hostResources, err := getHostResources(hostStats, node)
		if err != nil {
			c.Ui.Output("")
			c.Ui.Error(fmt.Sprintf("error fetching node stats: %v", err))
		}
		if err == nil {
			c.Ui.Output(c.Colorize().Color("\n[bold]Host Resource Utilization[reset]"))
			c.Ui.Output(formatList(hostResources))
		}

		if hostStats != nil && c.stats {
			c.Ui.Output(c.Colorize().Color("\n[bold]CPU Stats[reset]"))
			c.printCpuStats(hostStats)
			c.Ui.Output(c.Colorize().Color("\n[bold]Memory Stats[reset]"))
			c.printMemoryStats(hostStats)
			c.Ui.Output(c.Colorize().Color("\n[bold]Disk Stats[reset]"))
			c.printDiskStats(hostStats)
		}
	}

	nodeAllocs, _, err := client.Nodes().Allocations(node.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node allocations: %s", err))
		return 1
	}

	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocList(nodeAllocs, c.verbose, c.length))

	if c.verbose {
		c.formatAttributes(node)
		c.formatMeta(node)
	}
	return 0

}

func (c *NodeStatusCommand) outputTruncatedNodeDriverInfo(node *api.Node) string {
	drivers := make([]string, 0, len(node.Drivers))

	for driverName, driverInfo := range node.Drivers {
		if !driverInfo.Detected {
			continue
		}

		if !driverInfo.Healthy {
			drivers = append(drivers, fmt.Sprintf("%s (unhealthy)", driverName))
		} else {
			drivers = append(drivers, driverName)
		}
	}
	sort.Strings(drivers)
	return strings.Trim(strings.Join(drivers, ","), ", ")
}

func (c *NodeStatusCommand) outputNodeDriverInfo(node *api.Node) {
	c.Ui.Output(c.Colorize().Color("\n[bold]Drivers"))

	size := len(node.Drivers)
	nodeDrivers := make([]string, 0, size+1)

	nodeDrivers = append(nodeDrivers, "Driver|Detected|Healthy|Message|Time")

	drivers := make([]string, 0, len(node.Drivers))
	for driver := range node.Drivers {
		drivers = append(drivers, driver)
	}
	sort.Strings(drivers)

	for _, driver := range drivers {
		info := node.Drivers[driver]
		timestamp := formatTime(info.UpdateTime)
		nodeDrivers = append(nodeDrivers, fmt.Sprintf("%s|%v|%v|%s|%s", driver, info.Detected, info.Healthy, info.HealthDescription, timestamp))
	}
	c.Ui.Output(formatList(nodeDrivers))
}

func (c *NodeStatusCommand) outputNodeStatusEvents(node *api.Node) {
	c.Ui.Output(c.Colorize().Color("\n[bold]Node Events"))
	c.outputNodeEvent(node.Events)
}

func (c *NodeStatusCommand) outputNodeEvent(events []*api.NodeEvent) {
	size := len(events)
	nodeEvents := make([]string, size+1)
	if c.verbose {
		nodeEvents[0] = "Time|Subsystem|Message|Details"
	} else {
		nodeEvents[0] = "Time|Subsystem|Message"
	}

	for i, event := range events {
		timestamp := formatTime(event.Timestamp)
		subsystem := formatEventSubsystem(event.Subsystem, event.Details["driver"])
		msg := event.Message
		if c.verbose {
			details := formatEventDetails(event.Details)
			nodeEvents[size-i] = fmt.Sprintf("%s|%s|%s|%s", timestamp, subsystem, msg, details)
		} else {
			nodeEvents[size-i] = fmt.Sprintf("%s|%s|%s", timestamp, subsystem, msg)
		}
	}
	c.Ui.Output(formatList(nodeEvents))
}

func formatEventSubsystem(subsystem, driverName string) string {
	if driverName == "" {
		return subsystem
	}

	// If this event is for a driver, append the driver name to make the message
	// clearer
	return fmt.Sprintf("Driver: %s", driverName)
}

func formatEventDetails(details map[string]string) string {
	output := make([]string, 0, len(details))
	for k, v := range details {
		output = append(output, fmt.Sprintf("%s: %s ", k, v))
	}
	return strings.Join(output, ", ")
}

func (c *NodeStatusCommand) formatAttributes(node *api.Node) {
	// Print the attributes
	keys := make([]string, len(node.Attributes))
	for k := range node.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var attributes []string
	for _, k := range keys {
		if k != "" {
			attributes = append(attributes, fmt.Sprintf("%s|%s", k, node.Attributes[k]))
		}
	}
	c.Ui.Output(c.Colorize().Color("\n[bold]Attributes[reset]"))
	c.Ui.Output(formatKV(attributes))
}

func (c *NodeStatusCommand) formatMeta(node *api.Node) {
	// Print the meta
	keys := make([]string, 0, len(node.Meta))
	for k := range node.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var meta []string
	for _, k := range keys {
		if k != "" {
			meta = append(meta, fmt.Sprintf("%s|%s", k, node.Meta[k]))
		}
	}
	c.Ui.Output(c.Colorize().Color("\n[bold]Meta[reset]"))
	c.Ui.Output(formatKV(meta))
}

func (c *NodeStatusCommand) printCpuStats(hostStats *api.HostStats) {
	l := len(hostStats.CPU)
	for i, cpuStat := range hostStats.CPU {
		cpuStatsAttr := make([]string, 4)
		cpuStatsAttr[0] = fmt.Sprintf("CPU|%v", cpuStat.CPU)
		cpuStatsAttr[1] = fmt.Sprintf("User|%v%%", humanize.FormatFloat(floatFormat, cpuStat.User))
		cpuStatsAttr[2] = fmt.Sprintf("System|%v%%", humanize.FormatFloat(floatFormat, cpuStat.System))
		cpuStatsAttr[3] = fmt.Sprintf("Idle|%v%%", humanize.FormatFloat(floatFormat, cpuStat.Idle))
		c.Ui.Output(formatKV(cpuStatsAttr))
		if i+1 < l {
			c.Ui.Output("")
		}
	}
}

func (c *NodeStatusCommand) printMemoryStats(hostStats *api.HostStats) {
	memoryStat := hostStats.Memory
	memStatsAttr := make([]string, 4)
	memStatsAttr[0] = fmt.Sprintf("Total|%v", humanize.IBytes(memoryStat.Total))
	memStatsAttr[1] = fmt.Sprintf("Available|%v", humanize.IBytes(memoryStat.Available))
	memStatsAttr[2] = fmt.Sprintf("Used|%v", humanize.IBytes(memoryStat.Used))
	memStatsAttr[3] = fmt.Sprintf("Free|%v", humanize.IBytes(memoryStat.Free))
	c.Ui.Output(formatKV(memStatsAttr))
}

func (c *NodeStatusCommand) printDiskStats(hostStats *api.HostStats) {
	l := len(hostStats.DiskStats)
	for i, diskStat := range hostStats.DiskStats {
		diskStatsAttr := make([]string, 7)
		diskStatsAttr[0] = fmt.Sprintf("Device|%s", diskStat.Device)
		diskStatsAttr[1] = fmt.Sprintf("MountPoint|%s", diskStat.Mountpoint)
		diskStatsAttr[2] = fmt.Sprintf("Size|%s", humanize.IBytes(diskStat.Size))
		diskStatsAttr[3] = fmt.Sprintf("Used|%s", humanize.IBytes(diskStat.Used))
		diskStatsAttr[4] = fmt.Sprintf("Available|%s", humanize.IBytes(diskStat.Available))
		diskStatsAttr[5] = fmt.Sprintf("Used Percent|%v%%", humanize.FormatFloat(floatFormat, diskStat.UsedPercent))
		diskStatsAttr[6] = fmt.Sprintf("Inodes Percent|%v%%", humanize.FormatFloat(floatFormat, diskStat.InodesUsedPercent))
		c.Ui.Output(formatKV(diskStatsAttr))
		if i+1 < l {
			c.Ui.Output("")
		}
	}
}

// getRunningAllocs returns a slice of allocation id's running on the node
func getRunningAllocs(client *api.Client, nodeID string) ([]*api.Allocation, error) {
	var allocs []*api.Allocation

	// Query the node allocations
	nodeAllocs, _, err := client.Nodes().Allocations(nodeID, nil)
	// Filter list to only running allocations
	for _, alloc := range nodeAllocs {
		if alloc.ClientStatus == "running" {
			allocs = append(allocs, alloc)
		}
	}
	return allocs, err
}

// getAllocatedResources returns the resource usage of the node.
func getAllocatedResources(client *api.Client, runningAllocs []*api.Allocation, node *api.Node) []string {
	// Compute the total
	total := computeNodeTotalResources(node)

	// Get Resources
	var cpu, mem, disk, iops int
	for _, alloc := range runningAllocs {
		cpu += *alloc.Resources.CPU
		mem += *alloc.Resources.MemoryMB
		disk += *alloc.Resources.DiskMB
		iops += *alloc.Resources.IOPS
	}

	resources := make([]string, 2)
	resources[0] = "CPU|Memory|Disk|IOPS"
	resources[1] = fmt.Sprintf("%d/%d MHz|%s/%s|%s/%s|%d/%d",
		cpu,
		*total.CPU,
		humanize.IBytes(uint64(mem*bytesPerMegabyte)),
		humanize.IBytes(uint64(*total.MemoryMB*bytesPerMegabyte)),
		humanize.IBytes(uint64(disk*bytesPerMegabyte)),
		humanize.IBytes(uint64(*total.DiskMB*bytesPerMegabyte)),
		iops,
		*total.IOPS)

	return resources
}

// computeNodeTotalResources returns the total allocatable resources (resources
// minus reserved)
func computeNodeTotalResources(node *api.Node) api.Resources {
	total := api.Resources{}

	r := node.Resources
	res := node.Reserved
	if res == nil {
		res = &api.Resources{}
	}
	total.CPU = helper.IntToPtr(*r.CPU - *res.CPU)
	total.MemoryMB = helper.IntToPtr(*r.MemoryMB - *res.MemoryMB)
	total.DiskMB = helper.IntToPtr(*r.DiskMB - *res.DiskMB)
	total.IOPS = helper.IntToPtr(*r.IOPS - *res.IOPS)
	return total
}

// getActualResources returns the actual resource usage of the allocations.
func getActualResources(client *api.Client, runningAllocs []*api.Allocation, node *api.Node) ([]string, error) {
	// Compute the total
	total := computeNodeTotalResources(node)

	// Get Resources
	var cpu float64
	var mem uint64
	for _, alloc := range runningAllocs {
		// Make the call to the client to get the actual usage.
		stats, err := client.Allocations().Stats(alloc, nil)
		if err != nil {
			return nil, err
		}

		cpu += stats.ResourceUsage.CpuStats.TotalTicks
		mem += stats.ResourceUsage.MemoryStats.RSS
	}

	resources := make([]string, 2)
	resources[0] = "CPU|Memory"
	resources[1] = fmt.Sprintf("%v/%d MHz|%v/%v",
		math.Floor(cpu),
		*total.CPU,
		humanize.IBytes(mem),
		humanize.IBytes(uint64(*total.MemoryMB*bytesPerMegabyte)))

	return resources, nil
}

// getHostResources returns the actual resource usage of the node.
func getHostResources(hostStats *api.HostStats, node *api.Node) ([]string, error) {
	if hostStats == nil {
		return nil, fmt.Errorf("actual resource usage not present")
	}
	var resources []string

	// calculate disk usage
	storageDevice := node.Attributes["unique.storage.volume"]
	var diskUsed, diskSize uint64
	var physical bool
	for _, disk := range hostStats.DiskStats {
		if disk.Device == storageDevice {
			diskUsed = disk.Used
			diskSize = disk.Size
			physical = true
		}
	}

	resources = make([]string, 2)
	resources[0] = "CPU|Memory|Disk"
	if physical {
		resources[1] = fmt.Sprintf("%v/%d MHz|%s/%s|%s/%s",
			math.Floor(hostStats.CPUTicksConsumed),
			*node.Resources.CPU,
			humanize.IBytes(hostStats.Memory.Used),
			humanize.IBytes(hostStats.Memory.Total),
			humanize.IBytes(diskUsed),
			humanize.IBytes(diskSize),
		)
	} else {
		// If non-physical device are used, output device name only,
		// since nomad doesn't collect the stats data.
		resources[1] = fmt.Sprintf("%v/%d MHz|%s/%s|(%s)",
			math.Floor(hostStats.CPUTicksConsumed),
			*node.Resources.CPU,
			humanize.IBytes(hostStats.Memory.Used),
			humanize.IBytes(hostStats.Memory.Total),
			storageDevice,
		)
	}
	return resources, nil
}

// formatNodeStubList is used to return a table format of a list of node stubs.
func formatNodeStubList(nodes []*api.NodeListStub, verbose bool) string {
	// Return error if no nodes are found
	if len(nodes) == 0 {
		return ""
	}
	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Format the nodes list that matches the prefix so that the user
	// can create a more specific request
	out := make([]string, len(nodes)+1)
	out[0] = "ID|DC|Name|Class|Drain|Eligibility|Status"
	for i, node := range nodes {
		out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%s|%s",
			limit(node.ID, length),
			node.Datacenter,
			node.Name,
			node.NodeClass,
			node.Drain,
			node.SchedulingEligibility,
			node.Status)
	}

	return formatList(out)
}
