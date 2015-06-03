package nomad

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

// nomadFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type nomadFSM struct {
	logOutput io.Writer
	logger    *log.Logger
	state     *StateStore
}

// nomadSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type nomadSnapshot struct {
	state *StateSnapshot
}

// NewFSMPath is used to construct a new FSM with a blank state
func NewFSM(logOutput io.Writer) (*nomadFSM, error) {
	// Create a state store
	state, err := NewStateStore(logOutput)
	if err != nil {
		return nil, err
	}

	fsm := &nomadFSM{
		logOutput: logOutput,
		logger:    log.New(logOutput, "", log.LstdFlags),
		state:     state,
	}
	return fsm, nil
}

// Close is used to cleanup resources associated with the FSM
func (n *nomadFSM) Close() error {
	return n.state.Close()
}

// State is used to return a handle to the current state
func (n *nomadFSM) State() *StateStore {
	return n.state
}

func (n *nomadFSM) Apply(log *raft.Log) interface{} {
	buf := log.Data
	msgType := structs.MessageType(buf[0])

	// Check if this message type should be ignored when unknown. This is
	// used so that new commands can be added with developer control if older
	// versions can safely ignore the command, or if they should crash.
	ignoreUnknown := false
	if msgType&structs.IgnoreUnknownTypeFlag == structs.IgnoreUnknownTypeFlag {
		msgType &= ^structs.IgnoreUnknownTypeFlag
		ignoreUnknown = true
	}

	switch msgType {
	default:
		if ignoreUnknown {
			n.logger.Printf("[WARN] nomad.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
			return nil
		} else {
			panic(fmt.Errorf("failed to apply request: %#v", buf))
		}
	}
}
func (n *nomadFSM) Snapshot() (raft.FSMSnapshot, error) {
	// Create a new snapshot
	snap, err := n.state.Snapshot()
	if err != nil {
		return nil, err
	}
	return &nomadSnapshot{snap}, nil
}

func (n *nomadFSM) Restore(old io.ReadCloser) error {
	defer old.Close()
	return nil
}

func (s *nomadSnapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "persist"}, time.Now())
	return nil
}

func (s *nomadSnapshot) Release() {
	s.state.Close()
}
