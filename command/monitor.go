// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
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
	status     string
	desc       string
	node       string
	deployment string
	job        string
	allocs     map[string]*allocState
	wait       time.Duration
	index      uint64
}

// newEvalState creates and initializes a new monitorState
func newEvalState() *evalState {
	return &evalState{
		status: api.EvalStatusPending,
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
	if colorUi, ok := ui.(*cli.ColoredUi); ok {
		// Disable Info color for monitored output
		ui = &cli.ColoredUi{
			ErrorColor: colorUi.ErrorColor,
			WarnColor:  colorUi.WarnColor,
			InfoColor:  cli.UiColorNone,
			Ui:         colorUi.Ui,
		}
	}
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
		m.ui.Output(fmt.Sprintf("%s: Evaluation triggered by node %q",
			formatTime(time.Now()), limit(update.node, m.length)))
	}

	// Check if the evaluation was triggered by a job
	if existing.job == "" && update.job != "" {
		m.ui.Output(fmt.Sprintf("%s: Evaluation triggered by job %q",
			formatTime(time.Now()), update.job))
	}

	// Check if the evaluation was triggered by a deployment
	if existing.deployment == "" && update.deployment != "" {
		m.ui.Output(fmt.Sprintf("%s: Evaluation within deployment: %q",
			formatTime(time.Now()), limit(update.deployment, m.length)))
	}

	// Check the allocations
	for allocID, alloc := range update.allocs {
		if existing, ok := existing.allocs[allocID]; !ok {
			switch {
			case alloc.index < update.index:
				// New alloc with create index lower than the eval
				// create index indicates modification
				m.ui.Output(fmt.Sprintf(
					"%s: Allocation %q modified: node %q, group %q",
					formatTime(time.Now()), limit(alloc.id, m.length),
					limit(alloc.node, m.length), alloc.group))

			case alloc.desired == api.AllocDesiredStatusRun:
				// New allocation with desired status running
				m.ui.Output(fmt.Sprintf(
					"%s: Allocation %q created: node %q, group %q",
					formatTime(time.Now()), limit(alloc.id, m.length),
					limit(alloc.node, m.length), alloc.group))
			}
		} else {
			switch {
			case existing.client != alloc.client:
				description := ""
				if alloc.clientDesc != "" {
					description = fmt.Sprintf(" (%s)", alloc.clientDesc)
				}
				// Allocation status has changed
				m.ui.Output(fmt.Sprintf(
					"%s: Allocation %q status changed: %q -> %q%s",
					formatTime(time.Now()), limit(alloc.id, m.length),
					existing.client, alloc.client, description))
			}
		}
	}

	// Check if the status changed. We skip any transitions to pending status.
	if existing.status != "" &&
		update.status != api.AllocClientStatusPending &&
		existing.status != update.status {
		m.ui.Output(fmt.Sprintf("%s: Evaluation status changed: %q -> %q",
			formatTime(time.Now()), existing.status, update.status))
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

	// Add the initial pending state
	m.update(newEvalState())

	m.ui.Info(fmt.Sprintf("%s: Monitoring evaluation %q",
		formatTime(time.Now()), limit(evalID, m.length)))

	for {
		// Query the evaluation
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("No evaluation with id %q found", evalID))
			return 1
		}

		// Create the new eval state.
		state := newEvalState()
		state.status = eval.Status
		state.desc = eval.StatusDescription
		state.node = eval.NodeID
		state.job = eval.JobID
		state.deployment = eval.DeploymentID
		state.wait = eval.Wait
		state.index = eval.CreateIndex

		// Query the allocations associated with the evaluation
		allocs, _, err := m.client.Evaluations().Allocations(eval.ID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("%s: Error reading allocations: %s", formatTime(time.Now()), err))
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
		}

		// Update the state
		m.update(state)

		switch eval.Status {
		case api.EvalStatusComplete, api.EvalStatusFailed, api.EvalStatusCancelled:
			if len(eval.FailedTGAllocs) == 0 {
				m.ui.Info(fmt.Sprintf("%s: Evaluation %q finished with status %q",
					formatTime(time.Now()), limit(eval.ID, m.length), eval.Status))
			} else {
				// There were failures making the allocations
				schedFailure = true
				m.ui.Info(fmt.Sprintf("%s: Evaluation %q finished with status %q but failed to place all allocations:",
					formatTime(time.Now()), limit(eval.ID, m.length), eval.Status))

				// Print the failures per task group
				for tg, metrics := range eval.FailedTGAllocs {
					noun := "allocation"
					if metrics.CoalescedFailures > 0 {
						noun += "s"
					}
					m.ui.Output(fmt.Sprintf("%s: Task Group %q (failed to place %d %s):",
						formatTime(time.Now()), tg, metrics.CoalescedFailures+1, noun))
					metrics := formatAllocMetrics(metrics, false, "  ")
					for _, line := range strings.Split(metrics, "\n") {
						m.ui.Output(line)
					}
				}

				if eval.BlockedEval != "" {
					m.ui.Output(fmt.Sprintf("%s: Evaluation %q waiting for additional capacity to place remainder",
						formatTime(time.Now()), limit(eval.BlockedEval, m.length)))
				}
			}
		default:
			// Wait for the next update
			time.Sleep(updateWait)
			continue
		}

		// Monitor the next eval in the chain, if present
		if eval.NextEval != "" {
			if eval.Wait.Nanoseconds() != 0 {
				m.ui.Info(fmt.Sprintf(
					"%s: Monitoring next evaluation %q in %s",
					formatTime(time.Now()), limit(eval.NextEval, m.length), eval.Wait))

				// Skip some unnecessary polling
				time.Sleep(eval.Wait)
			}

			// Reset the state and monitor the new eval
			m.state = newEvalState()
			return m.monitor(eval.NextEval)
		}
		break
	}

	// Monitor the deployment if it exists
	dID := m.state.deployment
	if dID != "" {
		m.ui.Info(fmt.Sprintf("%s: Monitoring deployment %q", formatTime(time.Now()), limit(dID, m.length)))

		var verbose bool
		if m.length == fullId {
			verbose = true
		} else {
			verbose = false
		}

		meta := new(Meta)
		meta.Ui = m.ui
		cmd := &DeploymentStatusCommand{Meta: *meta}
		status, err := cmd.monitor(m.client, dID, 0, m.state.wait, verbose)
		if err != nil || status != api.DeploymentStatusSuccessful {
			return 1
		}
	}

	// Treat scheduling failures specially using a dedicated exit code.
	// This makes it easier to detect failures from the CLI.
	if schedFailure {
		return 2
	}

	return 0
}

func formatAllocMetrics(metrics *api.AllocationMetric, scores bool, prefix string) string {
	// Print a helpful message if we have an eligibility problem
	var out string
	if metrics.NodesEvaluated == 0 {
		out += fmt.Sprintf("%s* No nodes were eligible for evaluation\n", prefix)
	}

	// Print a helpful message if the user has asked for a DC that has no
	// available nodes.
	for dc, available := range metrics.NodesAvailable {
		if available == 0 {
			out += fmt.Sprintf("%s* No nodes are available in datacenter %q\n", prefix, dc)
		}
	}

	// Print filter info
	for class, num := range metrics.ClassFiltered {
		out += fmt.Sprintf("%s* Class %q: %d nodes excluded by filter\n", prefix, class, num)
	}
	for cs, num := range metrics.ConstraintFiltered {
		out += fmt.Sprintf("%s* Constraint %q: %d nodes excluded by filter\n", prefix, cs, num)
	}

	// Print exhaustion info
	if ne := metrics.NodesExhausted; ne > 0 {
		out += fmt.Sprintf("%s* Resources exhausted on %d nodes\n", prefix, ne)
	}
	for class, num := range metrics.ClassExhausted {
		out += fmt.Sprintf("%s* Class %q exhausted on %d nodes\n", prefix, class, num)
	}
	for dim, num := range metrics.DimensionExhausted {
		out += fmt.Sprintf("%s* Dimension %q exhausted on %d nodes\n", prefix, dim, num)
	}

	// Print quota info
	for _, dim := range metrics.QuotaExhausted {
		out += fmt.Sprintf("%s* Quota limit hit %q\n", prefix, dim)
	}

	// Print scores
	if scores {
		if len(metrics.ScoreMetaData) > 0 {
			scoreOutput := make([]string, len(metrics.ScoreMetaData)+1)

			// Find all possible scores and build header row.
			allScores := make(map[string]struct{})
			for _, scoreMeta := range metrics.ScoreMetaData {
				for score := range scoreMeta.Scores {
					allScores[score] = struct{}{}
				}
			}
			// Sort scores alphabetically.
			scores := make([]string, 0, len(allScores))
			for score := range allScores {
				scores = append(scores, score)
			}
			sort.Strings(scores)
			scoreOutput[0] = fmt.Sprintf("Node|%s|final score", strings.Join(scores, "|"))

			// Build row for each score.
			for i, scoreMeta := range metrics.ScoreMetaData {
				scoreOutput[i+1] = fmt.Sprintf("%v|", scoreMeta.NodeID)
				for _, scorerName := range scores {
					scoreVal := scoreMeta.Scores[scorerName]
					scoreOutput[i+1] += fmt.Sprintf("%.3g|", scoreVal)
				}
				scoreOutput[i+1] += fmt.Sprintf("%.3g", scoreMeta.NormScore)
			}

			out += formatList(scoreOutput)
		} else {
			// Backwards compatibility for old allocs
			for name, score := range metrics.Scores {
				out += fmt.Sprintf("%s* Score %q = %f\n", prefix, name, score)
			}
		}
	}

	out = strings.TrimSuffix(out, "\n")
	return out
}
