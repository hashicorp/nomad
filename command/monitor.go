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
	state  *monitorState

	sync.Mutex
}

// newMonitor returns a new monitor. The returned monitor will
// write output information to the provided ui.
func newMonitor(ui cli.Ui, client *api.Client) *monitor {
	return &monitor{
		ui:     ui,
		client: client,
		state:  new(monitorState),
	}
}

// output is used to write informational messages to the ui.
func (m *monitor) output(msg string) {
	m.ui.Output(fmt.Sprintf("%s %s", time.Now().Format(dateFmt), msg))
}

// monitorState is used to store the current "state of the world"
// in the context of monitoring an evaluation.
type monitorState struct {
	status string
	nodeID string
	wait   time.Duration
}

// update is used to update our monitor with new state. It can be
// called whether the passed information is new or not, and will
// only dump update messages when state changes.
func (m *monitor) update(eval *api.Evaluation) {
	m.Lock()
	defer m.Unlock()

	existing := m.state

	// Create the new state
	update := &monitorState{
		status: eval.Status,
		nodeID: eval.NodeID,
		wait:   eval.Wait,
	}
	defer func() { m.state = update }()

	// Check if the status changed
	if existing.status != update.status {
		m.output(fmt.Sprintf("Evaluation changed status from %q to %q",
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
// exit code for the command. The return code is 0 if monitoring
// succeeded and exited successfully, or 1 if an error was encountered
// or the eval status was returned as failed.
func (m *monitor) monitor(evalID string) int {
	for {
		// Check the current state of things
		eval, _, err := m.client.Evaluations().Info(evalID, nil)
		if err != nil {
			m.ui.Error(fmt.Sprintf("Error reading evaluation: %s", err))
			return 1
		}

		// Update the state
		m.update(eval)

		// Check if the eval is complete
		switch eval.Status {
		case structs.EvalStatusComplete:
			return 0
		case structs.EvalStatusFailed:
			return 1
		}

		// Wait for the next poll
		time.Sleep(time.Second)
	}

	return 0
}
