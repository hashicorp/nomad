// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/posener/complete"
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
	os          bool
	quiet       bool
	verbose     bool
	list_allocs bool
	self        bool
	stats       bool
	json        bool
	perPage     int
	pageToken   string
	filter      string
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

  If ACLs are enabled, this option requires a token with the 'node:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

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

  -per-page
    How many results to show per page.

  -page-token
    Where to start pagination.

  -filter
    Specifies an expression used to filter query results.

  -os
    Display operating system name.

  -quiet
    Display only node IDs.

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
			"-allocs":     complete.PredictNothing,
			"-filter":     complete.PredictAnything,
			"-json":       complete.PredictNothing,
			"-per-page":   complete.PredictAnything,
			"-page-token": complete.PredictAnything,
			"-self":       complete.PredictNothing,
			"-short":      complete.PredictNothing,
			"-stats":      complete.PredictNothing,
			"-t":          complete.PredictAnything,
			"-os":         complete.PredictAnything,
			"-quiet":      complete.PredictAnything,
			"-verbose":    complete.PredictNothing,
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

func (c *NodeStatusCommand) Name() string { return "node status" }

func (c *NodeStatusCommand) Run(args []string) int {

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.short, "short", false, "")
	flags.BoolVar(&c.os, "os", false, "")
	flags.BoolVar(&c.quiet, "quiet", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")
	flags.BoolVar(&c.list_allocs, "allocs", false, "")
	flags.BoolVar(&c.self, "self", false, "")
	flags.BoolVar(&c.stats, "stats", false, "")
	flags.BoolVar(&c.json, "json", false, "")
	flags.StringVar(&c.tmpl, "t", "", "")
	flags.StringVar(&c.filter, "filter", "", "")
	flags.IntVar(&c.perPage, "per-page", 0, "")
	flags.StringVar(&c.pageToken, "page-token", "", "")

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
		if c.quiet && (c.verbose || c.json) {
			c.Ui.Error("-quiet cannot be used with -verbose or -json")
			return 1
		}

		// Set up the options to capture any filter passed and pagination
		// details.
		opts := api.QueryOptions{
			Filter:    c.filter,
			PerPage:   int32(c.perPage),
			NextToken: c.pageToken,
		}

		// If the user requested showing the node OS, include this within the
		// query params.
		if c.os {
			opts.Params = map[string]string{"os": "true"}
		}

		// Query the node info
		nodes, qm, err := client.Nodes().List(&opts)
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
			c.Ui.Output("No nodes registered")
			return 0
		}

		var size int
		if c.quiet {
			size = len(nodes)
		} else {
			size = len(nodes) + 1
		}

		// Format the nodes list
		out := make([]string, size)

		if c.quiet {
			for i, node := range nodes {
				out[i] = node.ID
			}
			c.Ui.Output(formatList(out))
			return 0
		}

		out[0] = "ID|Node Pool|DC|Name|Class|"

		if c.os {
			out[0] += "OS|"
		}

		if c.verbose {
			out[0] += "Address|Version|"
		}

		out[0] += "Drain|Eligibility|Status"

		if c.list_allocs {
			out[0] += "|Running Allocs"
		}

		for i, node := range nodes {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s",
				limit(node.ID, c.length),
				node.NodePool,
				node.Datacenter,
				node.Name,
				node.NodeClass)
			if c.os {
				out[i+1] += fmt.Sprintf("|%s", node.Attributes["os.name"])
			}
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

		if qm.NextToken != "" {
			c.Ui.Output(fmt.Sprintf(`
Results have been paginated. To get the next page run:

%s -page-token %s`, argsWithoutPageToken(os.Args), qm.NextToken))
		}

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
		c.Ui.Error("Identifier must contain at least two characters.")
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

func nodeCSIControllerNames(n *api.Node) []string {
	var names []string
	for name := range n.CSIControllerPlugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func nodeCSINodeNames(n *api.Node) []string {
	var names []string
	for name := range n.CSINodePlugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func nodeCSIVolumeNames(allocs []*api.Allocation) []string {
	var names []string
	for _, alloc := range allocs {
		tg := alloc.GetTaskGroup()
		if tg == nil || len(tg.Volumes) == 0 {
			continue
		}

		for _, v := range tg.Volumes {
			if v.Type == api.CSIVolumeTypeCSI {
				names = append(names, v.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func nodeVolumeNames(n *api.Node) []string {
	var volumes []string
	for name := range n.HostVolumes {
		volumes = append(volumes, name)
	}

	sort.Strings(volumes)
	return volumes
}

func nodeNetworkNames(n *api.Node) []string {
	var networks []string
	for name := range n.HostNetworks {
		networks = append(networks, name)
	}

	sort.Strings(networks)
	return networks
}

func formatDrain(n *api.Node) string {
	if n.DrainStrategy != nil {
		b := new(strings.Builder)
		b.WriteString("true")
		if n.DrainStrategy.DrainSpec.Deadline.Nanoseconds() < 0 {
			b.WriteString("; force drain")
		} else if n.DrainStrategy.ForceDeadline.IsZero() {
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
	// Make one API call for allocations
	nodeAllocs, _, err := client.Nodes().Allocations(node.ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node allocations: %s", err))
		return 1
	}

	var runningAllocs []*api.Allocation
	for _, alloc := range nodeAllocs {
		if alloc.ClientStatus == "running" {
			runningAllocs = append(runningAllocs, alloc)
		}
	}

	// Format the header output
	basic := []string{
		fmt.Sprintf("ID|%s", node.ID),
		fmt.Sprintf("Name|%s", node.Name),
		fmt.Sprintf("Node Pool|%s", node.NodePool),
		fmt.Sprintf("Class|%s", node.NodeClass),
		fmt.Sprintf("DC|%s", node.Datacenter),
		fmt.Sprintf("Drain|%v", formatDrain(node)),
		fmt.Sprintf("Eligibility|%s", node.SchedulingEligibility),
		fmt.Sprintf("Status|%s", node.Status),
		fmt.Sprintf("CSI Controllers|%s", strings.Join(nodeCSIControllerNames(node), ",")),
		fmt.Sprintf("CSI Drivers|%s", strings.Join(nodeCSINodeNames(node), ",")),
	}

	if c.short {
		basic = append(basic, fmt.Sprintf("Host Volumes|%s", strings.Join(nodeVolumeNames(node), ",")))
		basic = append(basic, fmt.Sprintf("Host Networks|%s", strings.Join(nodeNetworkNames(node), ",")))
		basic = append(basic, fmt.Sprintf("CSI Volumes|%s", strings.Join(nodeCSIVolumeNames(runningAllocs), ",")))
		basic = append(basic, fmt.Sprintf("Drivers|%s", strings.Join(nodeDrivers(node), ",")))
		c.Ui.Output(c.Colorize().Color(formatKV(basic)))

		// Output alloc info
		if err := c.outputAllocInfo(node, nodeAllocs); err != nil {
			c.Ui.Error(fmt.Sprintf("%s", err))
			return 1
		}

		return 0
	}

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

	// When we're not running in verbose mode, then also include host volumes and
	// driver info in the basic output
	if !c.verbose {
		basic = append(basic, fmt.Sprintf("Host Volumes|%s", strings.Join(nodeVolumeNames(node), ",")))
		basic = append(basic, fmt.Sprintf("Host Networks|%s", strings.Join(nodeNetworkNames(node), ",")))
		basic = append(basic, fmt.Sprintf("CSI Volumes|%s", strings.Join(nodeCSIVolumeNames(runningAllocs), ",")))
		driverStatus := fmt.Sprintf("Driver Status| %s", c.outputTruncatedNodeDriverInfo(node))
		basic = append(basic, driverStatus)
	}

	// Output the basic info
	c.Ui.Output(c.Colorize().Color(formatKV(basic)))

	// If we're running in verbose mode, include full host volume and driver info
	if c.verbose {
		c.outputNodeVolumeInfo(node)
		c.outputNodeNetworkInfo(node)
		c.outputNodeCSIVolumeInfo(client, node, runningAllocs)
		c.outputNodeDriverInfo(node)
	}

	// Emit node events
	c.outputNodeStatusEvents(node)

	// Get list of running allocations on the node
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

	if err == nil && node.NodeResources != nil && len(node.NodeResources.Devices) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Device Resource Utilization[reset]"))
		c.Ui.Output(formatList(getDeviceResourcesForNode(hostStats.DeviceStats, node)))
	}
	if hostStats != nil && c.stats {
		c.Ui.Output(c.Colorize().Color("\n[bold]CPU Stats[reset]"))
		c.printCpuStats(hostStats)
		c.Ui.Output(c.Colorize().Color("\n[bold]Memory Stats[reset]"))
		c.printMemoryStats(hostStats)
		c.Ui.Output(c.Colorize().Color("\n[bold]Disk Stats[reset]"))
		c.printDiskStats(hostStats)
		if len(hostStats.DeviceStats) > 0 {
			c.Ui.Output(c.Colorize().Color("\n[bold]Device Stats[reset]"))
			printDeviceStats(c.Ui, hostStats.DeviceStats)
		}
	}

	if err := c.outputAllocInfo(node, nodeAllocs); err != nil {
		c.Ui.Error(fmt.Sprintf("%s", err))
		return 1
	}

	return 0
}

func (c *NodeStatusCommand) outputAllocInfo(node *api.Node, nodeAllocs []*api.Allocation) error {
	c.Ui.Output(c.Colorize().Color("\n[bold]Allocations[reset]"))
	c.Ui.Output(formatAllocList(nodeAllocs, c.verbose, c.length))

	if c.verbose {
		c.formatAttributes(node)
		c.formatDeviceAttributes(node)
		c.formatMeta(node)
	}

	return nil
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

func (c *NodeStatusCommand) outputNodeVolumeInfo(node *api.Node) {

	names := make([]string, 0, len(node.HostVolumes))
	for name := range node.HostVolumes {
		names = append(names, name)
	}
	sort.Strings(names)

	output := make([]string, 0, len(names)+1)
	output = append(output, "Name|ReadOnly|Source")

	if len(names) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Host Volumes"))
		for _, volName := range names {
			info := node.HostVolumes[volName]
			output = append(output, fmt.Sprintf("%s|%v|%s", volName, info.ReadOnly, info.Path))
		}
		c.Ui.Output(formatList(output))
	}
}

func (c *NodeStatusCommand) outputNodeNetworkInfo(node *api.Node) {

	names := make([]string, 0, len(node.HostNetworks))
	for name := range node.HostNetworks {
		names = append(names, name)
	}
	sort.Strings(names)

	output := make([]string, 0, len(names)+1)
	output = append(output, "Name|CIDR|Interface|ReservedPorts")

	if len(names) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Host Networks"))
		for _, hostNetworkName := range names {
			info := node.HostNetworks[hostNetworkName]
			output = append(output, fmt.Sprintf("%s|%v|%s|%s", hostNetworkName, info.CIDR, info.Interface, info.ReservedPorts))
		}
		c.Ui.Output(formatList(output))
	}
}

func (c *NodeStatusCommand) outputNodeCSIVolumeInfo(client *api.Client, node *api.Node, runningAllocs []*api.Allocation) {

	// Duplicate nodeCSIVolumeNames to sort by name but also index volume names to ids
	var names []string
	requests := map[string]*api.VolumeRequest{}
	for _, alloc := range runningAllocs {
		tg := alloc.GetTaskGroup()
		if tg == nil || len(tg.Volumes) == 0 {
			continue
		}

		for _, v := range tg.Volumes {
			if v.Type == api.CSIVolumeTypeCSI {
				names = append(names, v.Name)
				requests[v.Source] = v
			}
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)

	// Fetch the volume objects with current status
	// Ignore an error, all we're going to do is omit the volumes
	volumes := map[string]*api.CSIVolumeListStub{}
	vs, _ := client.Nodes().CSIVolumes(node.ID, &api.QueryOptions{
		Namespace: "*",
	})
	for _, v := range vs {
		n, ok := requests[v.ID]
		if ok {
			volumes[n.Name] = v
		}
	}

	if len(names) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]CSI Volumes"))

		// Output the volumes in name order
		output := make([]string, 0, len(names)+1)
		output = append(output, "ID|Name|Namespace|Plugin ID|Schedulable|Provider|Access Mode")
		for _, name := range names {
			v, ok := volumes[name]
			if ok {
				output = append(output, fmt.Sprintf(
					"%s|%s|%s|%s|%t|%s|%s",
					v.ID,
					name,
					v.Namespace,
					v.PluginID,
					v.Schedulable,
					v.Provider,
					v.AccessMode,
				))
			}
		}

		c.Ui.Output(formatList(output))
	}
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
		output = append(output, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(output, ", ")
}

func (c *NodeStatusCommand) formatAttributes(node *api.Node) {
	keys := make([]string, 0, len(node.Attributes))
	for k := range node.Attributes {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var attributes []string
	for _, k := range keys {
		attributes = append(attributes, fmt.Sprintf("%s|%s", k, node.Attributes[k]))
	}
	c.Ui.Output(c.Colorize().Color("\n[bold]Attributes[reset]"))
	c.Ui.Output(formatKV(attributes))
}

func (c *NodeStatusCommand) formatDeviceAttributes(node *api.Node) {
	if node.NodeResources == nil {
		return
	}
	devices := node.NodeResources.Devices
	if len(devices) == 0 {
		return
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID() < devices[j].ID()
	})

	first := true
	for _, d := range devices {
		if len(d.Attributes) == 0 {
			continue
		}

		if first {
			c.Ui.Output(c.Colorize().Color("\n[bold]Device Group Attributes[reset]"))
			first = false
		} else {
			c.Ui.Output("")
		}
		c.Ui.Output(formatKV(getDeviceAttributes(d)))
	}
}

func (c *NodeStatusCommand) formatMeta(node *api.Node) {
	c.Ui.Output(c.Colorize().Color("\n[bold]Meta[reset]"))
	c.Ui.Output(formatNodeMeta(node.Meta))
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
	var cpu, mem, disk int
	for _, alloc := range runningAllocs {
		cpu += *alloc.Resources.CPU
		mem += *alloc.Resources.MemoryMB
		disk += *alloc.Resources.DiskMB
	}

	resources := make([]string, 2)
	resources[0] = "CPU|Memory|Disk"
	resources[1] = fmt.Sprintf("%d/%d MHz|%s/%s|%s/%s",
		cpu,
		*total.CPU,
		humanize.IBytes(uint64(mem*bytesPerMegabyte)),
		humanize.IBytes(uint64(*total.MemoryMB*bytesPerMegabyte)),
		humanize.IBytes(uint64(disk*bytesPerMegabyte)),
		humanize.IBytes(uint64(*total.DiskMB*bytesPerMegabyte)))

	return resources
}

// computeNodeTotalResources returns the total allocatable resources (resources
// minus reserved)
func computeNodeTotalResources(node *api.Node) api.Resources {
	total := api.Resources{}

	r := node.NodeResources
	res := node.ReservedResources

	total.CPU = pointer.Of[int](int(r.Cpu.CpuShares) - int(res.Cpu.CpuShares))
	total.MemoryMB = pointer.Of[int](int(r.Memory.MemoryMB) - int(res.Memory.MemoryMB))
	total.DiskMB = pointer.Of[int](int(r.Disk.DiskMB) - int(res.Disk.DiskMB))
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
		if stats.ResourceUsage.MemoryStats.Usage > 0 {
			mem += stats.ResourceUsage.MemoryStats.Usage
		} else {
			mem += stats.ResourceUsage.MemoryStats.RSS
		}
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
			node.NodeResources.Cpu.CpuShares,
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
			node.NodeResources.Cpu.CpuShares,
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
