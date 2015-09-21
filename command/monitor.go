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
	nodeID string
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
func (m *monitor) update(eval *api.Evaluation, allocs []*api.AllocationListStub) {
	m.Lock()
	defer m.Unlock()

	existing := m.state

	// Create the new state
	update := &evalState{
		status: eval.Status,
		desc:   eval.StatusDescription,
		nodeID: eval.NodeID,
		allocs: make(map[string]*allocState),
		wait:   eval.Wait,
		index:  eval.CreateIndex,
	}
	for _, alloc := range allocs {
		update.allocs[alloc.ID] = &allocState{
			id:          alloc.ID,
			group:       alloc.TaskGroup,
			node:        alloc.NodeID,
			desired:     alloc.DesiredStatus,
			desiredDesc: alloc.DesiredDescription,
			client:      alloc.ClientStatus,
			index:       alloc.CreateIndex,
		}
	}
	defer func() { m.state = update }()

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
				fullAlloc, _, err := m.client.Allocations().Info(allocID, nil)
				if err != nil {
					m.ui.Output(fmt.Sprintf("Error querying alloc: %s", err))
					continue
				}
				dumpAllocStatus(m.ui, fullAlloc)

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
			existing.status, eval.Status))
	}

	// Check if the wait time is different
	if existing.wait == 0 && update.wait != 0 {
		m.ui.Output(fmt.Sprintf("Waiting %s before running eval",
			eval.Wait))
	}

	// Check if the nodeID changed
	if existing.nodeID == "" && update.nodeID != "" {
		m.ui.Output(fmt.Sprintf("Evaluation was assigned node ID %q",
			eval.NodeID))
	}
}

// monitor is used to start monitoring the given evaluation ID. It
// writes output directly to the monitor's ui, and returns the
// exit code for the command. The return code indicates monitoring
// success or failure ONLY. It is no indication of the outcome of
// the evaluation, since conflating these values obscures things.
func (m *monitor) monitor(evalID string) int {
	m.ui.Info(fmt.Sprintf("Monitoring evaluation %q", evalID))
	for {
		// Query the evaluation
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
			return 1
		}

		// Query the allocations associated with the evaluation
		allocs, _, err := m.client.Evaluations().Allocations(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("Error reading allocations: %s", err))
			return 1
		}

		// Update the state
		m.update(eval, allocs)

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

	return 0
}
