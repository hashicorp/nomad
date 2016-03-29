package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

type NodeStatusCommand struct {
	Meta
}

func (c *NodeStatusCommand) Help() string {
	helpText := `
Usage: nomad node-status [options] <node>

  Display status information about a given node. The list of nodes
  returned includes only nodes which jobs may be scheduled to, and
  includes status and other high-level information.

  If a node ID is passed, information for that specific node will
  be displayed. If no node ID's are passed, then a short-hand
  list of all nodes will be displayed. The -self flag is useful to
  quickly access the status of the local node.

General Options:

  ` + generalOptionsUsage() + `

Node Status Options:

  -short
    Display short output. Used only when a single node is being
    queried, and drops verbose output about node allocations.

  -verbose
    Display full information.

  -self
    Query the status of the local node.

  -allocs
    Display a count of running allocations for each node.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeStatusCommand) Synopsis() string {
	return "Display status information about nodes"
}

func (c *NodeStatusCommand) Run(args []string) int {
	var short, verbose, list_allocs, self bool

	flags := c.Meta.FlagSet("node-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&list_allocs, "allocs", false, "")
	flags.BoolVar(&self, "self", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either a single node or none
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If -self flag is set then determine the current node.
	nodeID := ""
	if self {
		info, err := client.Agent().Self()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying agent info: %s", err))
			return 1
		}
		var stats map[string]interface{}
		stats, _ = info["stats"]
		clientStats, ok := stats["client"].(map[string]interface{})
		if !ok {
			c.Ui.Error("Nomad not running in client mode")
			return 1
		}

		nodeID, ok = clientStats["node_id"].(string)
		if !ok {
			c.Ui.Error("Failed to determine node ID")
			return 1
		}

	}

	// Use list mode if no node name was provided
	if len(args) == 0 && !self {
		// Query the node info
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node status: %s", err))
			return 1
		}

		// Return nothing if no nodes found
		if len(nodes) == 0 {
			return 0
		}

		// Format the nodes list
		out := make([]string, len(nodes)+1)
		if list_allocs {
			out[0] = "ID|DC|Name|Class|Drain|Status|Running Allocs"
		} else {
			out[0] = "ID|DC|Name|Class|Drain|Status"
		}
		for i, node := range nodes {
			if list_allocs {
				numAllocs, err := getRunningAllocs(client, node.ID)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error querying node allocations: %s", err))
					return 1
				}
				out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%s|%v",
					limit(node.ID, length),
					node.Datacenter,
					node.Name,
					node.NodeClass,
					node.Drain,
					node.Status,
					len(numAllocs))
			} else {
				out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%s",
					limit(node.ID, length),
					node.Datacenter,
					node.Name,
					node.NodeClass,
					node.Drain,
					node.Status)
			}
		}

		// Dump the output
		c.Ui.Output(formatList(out))
		return 0
	}

	// Query the specific node
	if !self {
		nodeID = args[0]
	}
	if len(nodeID) == 1 {
		c.Ui.Error(fmt.Sprintf("Identifier must contain at least two characters."))
		return 1
	}
	if len(nodeID)%2 == 1 {
		// Identifiers must be of even length, so we strip off the last byte
		// to provide a consistent user experience.
		nodeID = nodeID[:len(nodeID)-1]
	}

	// Exact lookup failed, try with prefix based search
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
		// Format the nodes list that matches the prefix so that the user
		// can create a more specific request
		out := make([]string, len(nodes)+1)
		out[0] = "ID|DC|Name|Class|Drain|Status"
		for i, node := range nodes {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%s",
				limit(node.ID, length),
				node.Datacenter,
				node.Name,
				node.NodeClass,
				node.Drain,
				node.Status)
		}
		// Dump the output
		c.Ui.Output(fmt.Sprintf("Prefix matched multiple nodes\n\n%s", formatList(out)))
		return 0
	}
	// Prefix lookup matched a single node
	node, _, err := client.Nodes().Info(nodes[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node info: %s", err))
		return 1
	}

	// Format the output
	basic := []string{
		fmt.Sprintf("ID|%s", limit(node.ID, length)),
		fmt.Sprintf("Name|%s", node.Name),
		fmt.Sprintf("Class|%s", node.NodeClass),
		fmt.Sprintf("DC|%s", node.Datacenter),
		fmt.Sprintf("Drain|%v", node.Drain),
		fmt.Sprintf("Status|%s", node.Status),
	}
	c.Ui.Output(formatKV(basic))

	if !short {
		resources, err := getResources(client, node)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node resources: %s", err))
			return 1
		}
		c.Ui.Output("\n==> Resource Utilization")
		c.Ui.Output(formatList(resources))

		allocs, err := getAllocs(client, node, length)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node allocations: %s", err))
			return 1
		}

		if len(allocs) > 1 {
			c.Ui.Output("\n==> Allocations")
			c.Ui.Output(formatList(allocs))
		}
	}

	if verbose {
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
		c.Ui.Output("\n==> Attributes")
		c.Ui.Output(formatKV(attributes))
	}

	return 0
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

// getAllocs returns information about every running allocation on the node
func getAllocs(client *api.Client, node *api.Node, length int) ([]string, error) {
	var allocs []string
	// Query the node allocations
	nodeAllocs, _, err := client.Nodes().Allocations(node.ID, nil)
	// Format the allocations
	allocs = make([]string, len(nodeAllocs)+1)
	allocs[0] = "ID|Eval ID|Job ID|Task Group|Desired Status|Client Status"
	for i, alloc := range nodeAllocs {
		allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			limit(alloc.ID, length),
			limit(alloc.EvalID, length),
			alloc.JobID,
			alloc.TaskGroup,
			alloc.DesiredStatus,
			alloc.ClientStatus)
	}
	return allocs, err
}

// getResources returns the resource usage of the node.
func getResources(client *api.Client, node *api.Node) ([]string, error) {
	var resources []string
	var cpu, mem, disk, iops int
	var totalCpu, totalMem, totalDisk, totalIops int

	// Compute the total
	r := node.Resources
	res := node.Reserved
	if res == nil {
		res = &api.Resources{}
	}
	totalCpu = r.CPU - res.CPU
	totalMem = r.MemoryMB - res.MemoryMB
	totalDisk = r.DiskMB - res.DiskMB
	totalIops = r.IOPS - res.IOPS

	// Get list of running allocations on the node
	runningAllocs, err := getRunningAllocs(client, node.ID)

	// Get Resources
	for _, alloc := range runningAllocs {
		cpu += alloc.Resources.CPU
		mem += alloc.Resources.MemoryMB
		disk += alloc.Resources.DiskMB
		iops += alloc.Resources.IOPS
	}

	resources = make([]string, 2)
	resources[0] = "CPU|Memory MB|Disk MB|IOPS"
	resources[1] = fmt.Sprintf("%v/%v|%v/%v|%v/%v|%v/%v",
		cpu,
		totalCpu,
		mem,
		totalMem,
		disk,
		totalDisk,
		iops,
		totalIops)

	return resources, err
}
