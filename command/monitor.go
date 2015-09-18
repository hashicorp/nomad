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
	// dateFmt is the format we use when printing the date in
	// status update messages during monitoring.
	dateFmt = "2006/01/02 15:04:05"
)

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
	return &monitor{
		ui: &cli.PrefixedUi{
			InfoPrefix:   "==> ",
			OutputPrefix: "    ",
			ErrorPrefix:  "==> ",
			Ui:           ui,
		},
		client: client,
		state: &evalState{
			allocs: make(map[string]*allocState),
		},
	}
}

// output is used to write informational messages to the ui.
func (m *monitor) output(msg string) {
	m.ui.Output(fmt.Sprintf("%s %s", time.Now().Format(dateFmt), msg))
}

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
				m.output(fmt.Sprintf("Scheduling error for group %q (%s)",
					alloc.group, alloc.desiredDesc))

			case alloc.index < update.index:
				// New alloc with create index lower than the eval
				// create index indicates modification
				m.output(fmt.Sprintf(
					"Allocation %q modified: node %q, group %q",
					alloc.id, alloc.node, alloc.group))

			case alloc.desired == structs.AllocDesiredStatusRun:
				// New allocation with desired status running
				m.output(fmt.Sprintf(
					"Allocation %q created: node %q, group %q",
					alloc.id, alloc.node, alloc.group))
			}
		} else {
			switch {
			case existing.client != alloc.client:
				// Allocation status has changed
				m.output(fmt.Sprintf(
					"Allocation %q status changed: %q -> %q",
					alloc.id, existing.client, alloc.client))
			}
		}
	}

	// Check if the status changed
	if existing.status != update.status {
		m.output(fmt.Sprintf("Evaluation status changed: %q -> %q",
			existing.status, eval.Status))
	}

	// Check if the wait time is different
	if existing.wait == 0 && update.wait != 0 {
		m.output(fmt.Sprintf("Waiting %s before running eval",
			eval.Wait))
	}

	// Check if the nodeID changed
	if existing.nodeID == "" && update.nodeID != "" {
		m.output(fmt.Sprintf("Evaluation was assigned node ID %q",
			eval.NodeID))
	}
}

// monitor is used to start monitoring the given evaluation ID. It
// writes output directly to the monitor's ui, and returns the
// exit code for the command. The return code indicates monitoring
// success or failure ONLY. It is no indication of the outcome of
// the evaluation, since conflating these values obscures things.
func (m *monitor) monitor(evalID string) int {
	// Check if the eval has already completed and fast-path it.
	eval, _, err := m.client.Evaluations().Info(evalID, nil)
	if err != nil {
		m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
		return 1
	}
	switch eval.Status {
	case structs.EvalStatusComplete, structs.EvalStatusFailed:
		m.ui.Info(fmt.Sprintf("Evaluation %q already finished with status %q",
			evalID, eval.Status))
		return 0
	}

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
			time.Sleep(time.Second)
			continue
		}

		// Monitor the next eval, if it exists.
		if eval.NextEval != "" {
			mon := newMonitor(m.ui, m.client)
			return mon.monitor(eval.NextEval)
		}
		break
	}

	return 0
}
