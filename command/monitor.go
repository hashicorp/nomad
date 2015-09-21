package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
)

const (
	// updateWait is the amount of time to wait between status
	// updates. Because the monitor is poll-based, we use this
	// delay to avoid overwhelming the API server.
	updateWait = time.Second
)

// evalState is used to store the current "state of the world"
// in the context of monitoring an evaluation.
type evalState struct {
	status string
	desc   string
	node   string
	allocs map[string]*allocState
	wait   time.Duration
	index  uint64
}

// allocState is used to track the state of an allocation
type allocState struct {
	id          string
	group       string
	node        string
	desired     string
	desiredDesc string
	client      string
	index       uint64

	// full is the allocation struct with full details. This
	// must be queried for explicitly so it is only included
	// if there is important error information inside.
	full *api.Allocation
}

// monitor wraps an evaluation monitor and holds metadata and
// state information.
type monitor struct {
	ui     cli.Ui
	client *api.Client
	state  *evalState

	sync.Mutex
}

// newMonitor returns a new monitor. The returned monitor will
// write output information to the provided ui.
func newMonitor(ui cli.Ui, client *api.Client) *monitor {
	mon := &monitor{
		ui: &cli.PrefixedUi{
			InfoPrefix:   "==> ",
			OutputPrefix: "    ",
			ErrorPrefix:  "==> ",
			Ui:           ui,
		},
		client: client,
	}
	mon.init()
	return mon
}

// init allocates substructures
func (m *monitor) init() {
	m.state = &evalState{
		allocs: make(map[string]*allocState),
	}
}

// update is used to update our monitor with new state. It can be
// called whether the passed information is new or not, and will
// only dump update messages when state changes.
func (m *monitor) update(update *evalState) {
	m.Lock()
	defer m.Unlock()

	existing := m.state

	// Swap in the new state at the end
	defer func() {
		m.state = update
	}()

	// Check the allocations
	for allocID, alloc := range update.allocs {
		if existing, ok := existing.allocs[allocID]; !ok {
			switch {
			case alloc.desired == structs.AllocDesiredStatusFailed:
				// New allocs with desired state failed indicate
				// scheduling failure.
				m.ui.Output(fmt.Sprintf("Scheduling error for group %q (%s)",
					alloc.group, alloc.desiredDesc))

				// Generate a more descriptive error for why the allocation
				// failed and dump it to the screen
				if alloc.full != nil {
					dumpAllocStatus(m.ui, alloc.full)
				}

			case alloc.index < update.index:
				// New alloc with create index lower than the eval
				// create index indicates modification
				m.ui.Output(fmt.Sprintf(
					"Allocation %q modified: node %q, group %q",
					alloc.id, alloc.node, alloc.group))

			case alloc.desired == structs.AllocDesiredStatusRun:
				// New allocation with desired status running
				m.ui.Output(fmt.Sprintf(
					"Allocation %q created: node %q, group %q",
					alloc.id, alloc.node, alloc.group))
			}
		} else {
			switch {
			case existing.client != alloc.client:
				// Allocation status has changed
				m.ui.Output(fmt.Sprintf(
					"Allocation %q status changed: %q -> %q",
					alloc.id, existing.client, alloc.client))
			}
		}
	}

	// Check if the status changed
	if existing.status != update.status {
		m.ui.Output(fmt.Sprintf("Evaluation status changed: %q -> %q",
			existing.status, update.status))
	}

	// Check if the wait time is different
	if existing.wait == 0 && update.wait != 0 {
		m.ui.Output(fmt.Sprintf("Waiting %s before running eval",
			update.wait))
	}

	// Check if the node changed
	if existing.node == "" && update.node != "" {
		m.ui.Output(fmt.Sprintf("Evaluation was assigned node ID %q",
			update.node))
	}
}

// monitor is used to start monitoring the given evaluation ID. It
// writes output directly to the monitor's ui, and returns the
// exit code for the command.
//
// The return code will be 0 on successful evaluation. If there are
// problems scheduling the job (impossible constraints, resources
// exhausted, etc), then the return code will be 2. For any other
// failures (API connectivity, internal errors, etc), the return code
// will be 1.
func (m *monitor) monitor(evalID string) int {
	// Track if we encounter a scheduling failure. This can only be
	// detected while querying allocations, so we use this bool to
	// carry that status into the return code.
	var schedFailure bool

	m.ui.Info(fmt.Sprintf("Monitoring evaluation %q", evalID))
	for {
		// Query the evaluation
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
			return 1
		}

		// Create the new eval state.
		state := &evalState{
			status: eval.Status,
			desc:   eval.StatusDescription,
			node:   eval.NodeID,
			allocs: make(map[string]*allocState),
			wait:   eval.Wait,
			index:  eval.CreateIndex,
		}

		// Query the allocations associated with the evaluation
		allocs, _, err := m.client.Evaluations().Allocations(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("Error reading allocations: %s", err))
			return 1
		}

		// Add the allocs to the state
		for _, alloc := range allocs {
			state.allocs[alloc.ID] = &allocState{
				id:          alloc.ID,
				group:       alloc.TaskGroup,
				node:        alloc.NodeID,
				desired:     alloc.DesiredStatus,
				desiredDesc: alloc.DesiredDescription,
				client:      alloc.ClientStatus,
				index:       alloc.CreateIndex,
			}

			// If we have a scheduling error, query the full allocation
			// to get the details.
			if alloc.DesiredStatus == structs.AllocDesiredStatusFailed {
				schedFailure = true
				failed, _, err := m.client.Allocations().Info(alloc.ID, nil)
				if err != nil {
					m.ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
					return 1
				}
				state.allocs[alloc.ID].full = failed
			}
		}

		// Update the state
		m.update(state)

		switch eval.Status {
		case structs.EvalStatusComplete, structs.EvalStatusFailed:
			m.ui.Info(fmt.Sprintf("Evaluation %q finished with status %q",
				eval.ID, eval.Status))
		default:
			// Wait for the next update
			time.Sleep(updateWait)
			continue
		}

		// Monitor the next eval, if it exists.
		if eval.NextEval != "" {
			m.init()
			return m.monitor(eval.NextEval)
		}
		break
	}

	// Treat scheduling failures specially using a dedicated exit code.
	// This makes it easier to detect failures from the CLI.
	if schedFailure {
		return 2
	}

	return 0
}

// dumpAllocStatus is a helper to generate a more user-friendly error message
// for scheduling failures, displaying a high level status of why the job
// could not be scheduled out.
func dumpAllocStatus(ui cli.Ui, alloc *api.Allocation) {
	// Print filter stats
	ui.Output(fmt.Sprintf("Allocation %q status %q (%d/%d nodes filtered)",
		alloc.ID, alloc.ClientStatus,
		alloc.Metrics.NodesFiltered, alloc.Metrics.NodesEvaluated))

	// Print exhaustion info
	if ne := alloc.Metrics.NodesExhausted; ne > 0 {
		ui.Output(fmt.Sprintf("  * Resources exhausted on %d nodes", ne))
	}
	for class, num := range alloc.Metrics.ClassExhausted {
		ui.Output(fmt.Sprintf("  * Class %q exhausted on %d nodes", class, num))
	}
	for dim, num := range alloc.Metrics.DimensionExhausted {
		ui.Output(fmt.Sprintf("  * Dimension %q exhausted on %d nodes", dim, num))
	}

	// Print filter info
	for class, num := range alloc.Metrics.ClassFiltered {
		ui.Output(fmt.Sprintf("  * Class %q filtered %d nodes", class, num))
	}
	for cs, num := range alloc.Metrics.ConstraintFiltered {
		ui.Output(fmt.Sprintf("  * Constraint %q filtered %d nodes", cs, num))
	}
}
