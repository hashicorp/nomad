package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
)

var (
	// defaultDrainDuration is the default drain duration if it is not specified
	// explicitly
	defaultDrainDuration = 1 * time.Hour
)

type NodeDrainCommand struct {
	Meta
}

func (c *NodeDrainCommand) Help() string {
	helpText := `
Usage: nomad node drain [options] <node>

  Toggles node draining on a specified node. It is required
  that either -enable or -disable is specified, but not both.
  The -self flag is useful to drain the local node.

General Options:

  ` + generalOptionsUsage() + `

Node Drain Options:

  -disable
    Disable draining for the specified node.

  -enable
    Enable draining for the specified node.

  -deadline <duration>
    Set the deadline by which all allocations must be moved off the node.
    Remaining allocations after the deadline are forced removed from the node.
    If unspecified, a default deadline of one hour is applied.

  -detach
    Return immediately instead of entering monitor mode.

  -force
    Force remove allocations off the node immediately.

  -no-deadline
    No deadline allows the allocations to drain off the node without being force
    stopped after a certain deadline.

  -ignore-system
    Ignore system allows the drain to complete without stopping system job
    allocations. By default system jobs are stopped last.

  -keep-ineligible
    Keep ineligible will maintain the node's scheduling ineligibility even if
    the drain is being disabled. This is useful when an existing drain is being
    cancelled but additional scheduling on the node is not desired.

  -self
    Set the drain status of the local node.

  -yes
    Automatic yes to prompts.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeDrainCommand) Synopsis() string {
	return "Toggle drain mode on a given node"
}

func (c *NodeDrainCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-disable":         complete.PredictNothing,
			"-enable":          complete.PredictNothing,
			"-deadline":        complete.PredictAnything,
			"-detach":          complete.PredictNothing,
			"-force":           complete.PredictNothing,
			"-no-deadline":     complete.PredictNothing,
			"-ignore-system":   complete.PredictNothing,
			"-keep-ineligible": complete.PredictNothing,
			"-self":            complete.PredictNothing,
			"-yes":             complete.PredictNothing,
		})
}

func (c *NodeDrainCommand) AutocompleteArgs() complete.Predictor {
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

func (c *NodeDrainCommand) Run(args []string) int {
	var enable, disable, detach, force,
		noDeadline, ignoreSystem, keepIneligible, self, autoYes bool
	var deadline string

	flags := c.Meta.FlagSet("node-drain", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&enable, "enable", false, "Enable drain mode")
	flags.BoolVar(&disable, "disable", false, "Disable drain mode")
	flags.StringVar(&deadline, "deadline", "", "Deadline after which allocations are force stopped")
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&force, "force", false, "Force immediate drain")
	flags.BoolVar(&noDeadline, "no-deadline", false, "Drain node with no deadline")
	flags.BoolVar(&ignoreSystem, "ignore-system", false, "Do not drain system job allocations from the node")
	flags.BoolVar(&keepIneligible, "keep-ineligible", false, "Do not update the nodes scheduling eligibility")
	flags.BoolVar(&self, "self", false, "")
	flags.BoolVar(&autoYes, "yes", false, "Automatic yes to prompts.")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either enable or disable, but not both.
	if (enable && disable) || (!enable && !disable) {
		c.Ui.Error(c.Help())
		return 1
	}

	// Check that we got a node ID
	args = flags.Args()
	if l := len(args); self && l != 0 || !self && l != 1 {
		c.Ui.Error("Node ID must be specified if -self isn't being used")
		return 1
	}

	// Validate a compatible set of flags were set
	if disable && (deadline != "" || force || noDeadline || ignoreSystem) {
		c.Ui.Error("-disable can't be combined with flags configuring drain strategy")
		return 1
	}
	if deadline != "" && (force || noDeadline) {
		c.Ui.Error("-deadline can't be combined with -force or -no-deadline")
		return 1
	}
	if force && noDeadline {
		c.Ui.Error("-force and -no-deadline are mutually exclusive")
		return 1
	}

	// Parse the duration
	var d time.Duration
	if force {
		d = -1 * time.Second
	} else if noDeadline {
		d = 0
	} else if deadline != "" {
		dur, err := time.ParseDuration(deadline)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse deadline %q: %v", deadline, err))
			return 1
		}
		if dur <= 0 {
			c.Ui.Error("A positive drain duration must be given")
			return 1
		}

		d = dur
	} else {
		d = defaultDrainDuration
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If -self flag is set then determine the current node.
	var nodeID string
	if !self {
		nodeID = args[0]
	} else {
		var err error
		if nodeID, err = getLocalNodeID(client); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	// Check if node exists
	if len(nodeID) == 1 {
		c.Ui.Error(fmt.Sprintf("Identifier must contain at least two characters."))
		return 1
	}

	nodeID = sanitizeUUIDPrefix(nodeID)
	nodes, _, err := client.Nodes().PrefixList(nodeID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling drain mode: %s", err))
		return 1
	}
	// Return error if no nodes are found
	if len(nodes) == 0 {
		c.Ui.Error(fmt.Sprintf("No node(s) with prefix or id %q found", nodeID))
		return 1
	}
	if len(nodes) > 1 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple nodes\n\n%s",
			formatNodeStubList(nodes, true)))
		return 1
	}

	// Prefix lookup matched a single node
	node, _, err := client.Nodes().Info(nodes[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling drain mode: %s", err))
		return 1
	}

	// Confirm drain if the node was a prefix match.
	if nodeID != node.ID && !autoYes {
		verb := "enable"
		if disable {
			verb = "disable"
		}
		question := fmt.Sprintf("Are you sure you want to %s drain mode for node %q? [y/N]", verb, node.ID)
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Canceling drain toggle")
			return 0
		} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
			// Non exact match yes
			c.Ui.Output("For confirmation, an exact ‘y’ is required.")
			return 0
		} else if answer != "y" {
			c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
			return 1
		}
	}

	var spec *api.DrainSpec
	if enable {
		spec = &api.DrainSpec{
			Deadline:         d,
			IgnoreSystemJobs: ignoreSystem,
		}
	}

	// Toggle node draining
	meta, err := client.Nodes().UpdateDrain(node.ID, spec, !keepIneligible, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating drain specification: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Node %q drain strategy set", node.ID))

	if enable && !detach {
		if err := monitorDrain(c.Ui.Output, client.Nodes(), node.ID, meta.LastIndex); err != nil {
			c.Ui.Error(fmt.Sprintf("Error monitoring drain: %v", err))
			return 1
		}

		c.Ui.Output(fmt.Sprintf("Node %q drain complete", nodeID))
	}

	return 0
}

// monitorDrain monitors the node being drained and exits when the node has
// finished draining.
func monitorDrain(output func(string), nodeClient *api.Nodes, nodeID string, index uint64) error {
	doneCh := make(chan struct{})
	defer close(doneCh)

	// Errors from either goroutine are sent here
	errCh := make(chan error, 1)

	// Monitor node changes and close chan when drain is complete
	nodeCh := make(chan struct{})
	go func() {
		for {
			q := api.QueryOptions{
				AllowStale: true,
				WaitIndex:  index,
			}
			node, meta, err := nodeClient.Info(nodeID, &q)
			if err != nil {
				select {
				case errCh <- err:
				case <-doneCh:
				}
				return
			}

			if node.DrainStrategy == nil {
				close(nodeCh)
				return
			}

			// Drain still ongoing
			index = meta.LastIndex
		}
	}()

	// Monitor alloc changes
	allocCh := make(chan string, 1)
	go func() {
		allocs, meta, err := nodeClient.Allocations(nodeID, nil)
		if err != nil {
			select {
			case errCh <- err:
			case <-doneCh:
			}
			return
		}

		initial := make(map[string]*api.Allocation, len(allocs))
		for _, a := range allocs {
			initial[a.ID] = a
		}

		for {
			q := api.QueryOptions{
				AllowStale: true,
				WaitIndex:  meta.LastIndex,
			}

			allocs, meta, err = nodeClient.Allocations(nodeID, &q)
			if err != nil {
				select {
				case errCh <- err:
				case <-doneCh:
				}
				return
			}

			for _, a := range allocs {
				// Get previous version of alloc
				orig, ok := initial[a.ID]

				// Update local alloc state
				initial[a.ID] = a

				migrating := a.DesiredTransition.ShouldMigrate()

				msg := ""
				switch {
				case !ok:
					// Should only be possible if response
					// from initial Allocations call was
					// stale. No need to output

				case orig.ClientStatus != a.ClientStatus:
					// Alloc status has changed; output
					msg = fmt.Sprintf("status %s -> %s", orig.ClientStatus, a.ClientStatus)

				case migrating && !orig.DesiredTransition.ShouldMigrate():
					// Alloc was marked for migration
					msg = "marked for migration"
				case migrating && (orig.DesiredStatus != a.DesiredStatus) && a.DesiredStatus == structs.AllocDesiredStatusStop:
					// Alloc has already been marked for migration and is now being stopped
					msg = "draining"
				case a.NextAllocation != "" && orig.NextAllocation == "":
					// Alloc has been replaced by another allocation
					msg = fmt.Sprintf("replaced by allocation %q", a.NextAllocation)
				}

				if msg != "" {
					select {
					case allocCh <- fmt.Sprintf("Alloc %q %s", a.ID, msg):
					case <-doneCh:
						return
					}
				}
			}
		}
	}()

	done := false
	for !done {
		select {
		case err := <-errCh:
			return err
		case <-nodeCh:
			done = true
		case msg := <-allocCh:
			output(msg)
		}
	}

	// Loop on alloc messages for a bit longer as we may have gotten the
	// "node done" first (since the watchers run concurrently the events
	// may be received out of order)
	deadline := 250 * time.Millisecond
	timer := time.NewTimer(deadline)
	for {
		select {
		case err := <-errCh:
			return err
		case msg := <-allocCh:
			output(msg)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(deadline)
		case <-timer.C:
			// No events within deadline, exit
			return nil
		}
	}
}
