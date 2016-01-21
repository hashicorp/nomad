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
	job    string
	allocs map[string]*allocState
	wait   time.Duration
	index  uint64
}

// newEvalState creates and initializes a new monitorState
func newEvalState() *evalState {
	return &evalState{
		status: structs.EvalStatusPending,
		allocs: make(map[string]*allocState),
	}
}

// allocState is used to track the state of an allocation
type allocState struct {
	id          string
	group       string
	node        string
	desired     string
	desiredDesc string
	client      string
	clientDesc  string
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

	// length determines the number of characters for identifiers in the ui.
	length int

	sync.Mutex
}

// newMonitor returns a new monitor. The returned monitor will
// write output information to the provided ui. The length parameter determines
// the number of characters for identifiers in the ui.
func newMonitor(ui cli.Ui, client *api.Client, length int) *monitor {
	mon := &monitor{
		ui: &cli.PrefixedUi{
			InfoPrefix:   "==> ",
			OutputPrefix: "    ",
			ErrorPrefix:  "==> ",
			Ui:           ui,
		},
		client: client,
		state:  newEvalState(),
		length: length,
	}
	return mon
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

	// Check if the evaluation was triggered by a node
	if existing.node == "" && update.node != "" {
		m.ui.Output(fmt.Sprintf("Evaluation triggered by node %q",
			update.node[:m.length]))
	}

	// Check if the evaluation was triggered by a job
	if existing.job == "" && update.job != "" {
		m.ui.Output(fmt.Sprintf("Evaluation triggered by job %q", update.job))
	}

	// Check the allocations
	for allocID, alloc := range update.allocs {
		if existing, ok := existing.allocs[allocID]; !ok {
			switch {
			case alloc.desired == structs.AllocDesiredStatusFailed:
				// New allocs with desired state failed indicate
				// scheduling failure.
				m.ui.Output(fmt.Sprintf("Scheduling error for group %q (%s)",
					alloc.group, alloc.desiredDesc))

				// Log the client status, if any provided
				if alloc.clientDesc != "" {
					m.ui.Output("Client reported status: " + alloc.clientDesc)
				}

				// Generate a more descriptive error for why the allocation
				// failed and dump it to the screen
				if alloc.full != nil {
					dumpAllocStatus(m.ui, alloc.full, m.length)
				}

			case alloc.index < update.index:
				// New alloc with create index lower than the eval
				// create index indicates modification
				m.ui.Output(fmt.Sprintf(
					"Allocation %q modified: node %q, group %q",
					alloc.id[:m.length], alloc.node[:m.length], alloc.group))

			case alloc.desired == structs.AllocDesiredStatusRun:
				// New allocation with desired status running
				m.ui.Output(fmt.Sprintf(
					"Allocation %q created: node %q, group %q",
					alloc.id[:m.length], alloc.node[:m.length], alloc.group))
			}
		} else {
			switch {
			case existing.client != alloc.client:
				// Allocation status has changed
				m.ui.Output(fmt.Sprintf(
					"Allocation %q status changed: %q -> %q (%s)",
					alloc.id[:m.length], existing.client, alloc.client, alloc.clientDesc))
			}
		}
	}

	// Check if the status changed. We skip any transitions to pending status.
	if existing.status != "" &&
		update.status != structs.AllocClientStatusPending &&
		existing.status != update.status {
		m.ui.Output(fmt.Sprintf("Evaluation status changed: %q -> %q",
			existing.status, update.status))
	}
}

// monitor is used to start monitoring the given evaluation ID. It
// writes output directly to the monitor's ui, and returns the
// exit code for the command. If allowPrefix is false, monitor will only accept
// exact matching evalIDs.
//
// The return code will be 0 on successful evaluation. If there are
// problems scheduling the job (impossible constraints, resources
// exhausted, etc), then the return code will be 2. For any other
// failures (API connectivity, internal errors, etc), the return code
// will be 1.
func (m *monitor) monitor(evalID string, allowPrefix bool) int {
	// Track if we encounter a scheduling failure. This can only be
	// detected while querying allocations, so we use this bool to
	// carry that status into the return code.
	var schedFailure bool

	// The user may have specified a prefix as eval id. We need to lookup the
	// full id from the database first. Since we do this in a loop we need a
	// variable to keep track if we've already written the header message.
	var headerWritten bool

	// Add the initial pending state
	m.update(newEvalState())

	for {
		// Query the evaluation
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			if !allowPrefix {
				m.ui.Error(fmt.Sprintf("No evaluation with id %q found", evalID))
				return 1
			}
			if len(evalID)%2 != 0 {
				m.ui.Error(fmt.Sprintf("Identifier (without hyphens) must be of even length."))
				return 1
			}

			evals, _, err := m.client.Evaluations().PrefixList(evalID)
			if err != nil {
				m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
				return 1
			}
			if len(evals) == 0 {
				m.ui.Error(fmt.Sprintf("No evaluation(s) with prefix or id %q found", evalID))
				return 1
			}
			if len(evals) > 1 {
				// Format the evaluations
				out := make([]string, len(evals)+1)
				out[0] = "ID|Priority|Type|TriggeredBy|Status"
				for i, eval := range evals {
					out[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s",
						eval.ID[:m.length],
						eval.Priority,
						eval.Type,
						eval.TriggeredBy,
						eval.Status)
				}
				m.ui.Output(fmt.Sprintf("Prefix matched multiple evaluations\n\n%s", formatList(out)))
				return 0
			}
			// Prefix lookup matched a single evaluation
			eval, _, err = m.client.Evaluations().Info(evals[0].ID, nil)
			if err != nil {
				m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
			}
		}

		if !headerWritten {
			m.ui.Info(fmt.Sprintf("Monitoring evaluation %q", eval.ID[:m.length]))
			headerWritten = true
		}

		// Create the new eval state.
		state := newEvalState()
		state.status = eval.Status
		state.desc = eval.StatusDescription
		state.node = eval.NodeID
		state.job = eval.JobID
		state.wait = eval.Wait
		state.index = eval.CreateIndex

		// Query the allocations associated with the evaluation
		allocs, _, err := m.client.Evaluations().Allocations(eval.ID, nil)
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
				clientDesc:  alloc.ClientDescription,
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
				eval.ID[:m.length], eval.Status))
		default:
			// Wait for the next update
			time.Sleep(updateWait)
			continue
		}

		// Monitor the next eval in the chain, if present
		if eval.NextEval != "" {
			m.ui.Info(fmt.Sprintf(
				"Monitoring next evaluation %q in %s",
				eval.NextEval, eval.Wait))

			// Skip some unnecessary polling
			time.Sleep(eval.Wait)

			// Reset the state and monitor the new eval
			m.state = newEvalState()
			return m.monitor(eval.NextEval, allowPrefix)
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
func dumpAllocStatus(ui cli.Ui, alloc *api.Allocation, length int) {
	// Print filter stats
	ui.Output(fmt.Sprintf("Allocation %q status %q (%d/%d nodes filtered)",
		alloc.ID[:length], alloc.ClientStatus,
		alloc.Metrics.NodesFiltered, alloc.Metrics.NodesEvaluated))

	// Print a helpful message if we have an eligibility problem
	if alloc.Metrics.NodesEvaluated == 0 {
		ui.Output("  * No nodes were eligible for evaluation")
	}

	// Print a helpful message if the user has asked for a DC that has no
	// available nodes.
	for dc, available := range alloc.Metrics.NodesAvailable {
		if available == 0 {
			ui.Output(fmt.Sprintf("  * No nodes are available in datacenter %q", dc))
		}
	}

	// Print filter info
	for class, num := range alloc.Metrics.ClassFiltered {
		ui.Output(fmt.Sprintf("  * Class %q filtered %d nodes", class, num))
	}
	for cs, num := range alloc.Metrics.ConstraintFiltered {
		ui.Output(fmt.Sprintf("  * Constraint %q filtered %d nodes", cs, num))
	}

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

	// Print scores
	for name, score := range alloc.Metrics.Scores {
		ui.Output(fmt.Sprintf("  * Score %q = %f", name, score))
	}
}
