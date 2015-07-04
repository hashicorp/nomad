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
	return nil
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
	case structs.RegisterRequestType:
		return n.decodeRegister(buf[1:], log.Index)
	case structs.DeregisterRequestType:
		return n.applyDeregister(buf[1:], log.Index)
	case structs.NodeUpdateStatusRequestType:
		return n.applyStatusUpdate(buf[1:], log.Index)
	default:
		if ignoreUnknown {
			n.logger.Printf("[WARN] nomad.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
			return nil
		} else {
			panic(fmt.Errorf("failed to apply request: %#v", buf))
		}
	}
}

func (n *nomadFSM) decodeRegister(buf []byte, index uint64) interface{} {
	var req structs.RegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	return n.applyRegister(&req, index)
}

func (n *nomadFSM) applyRegister(req *structs.RegisterRequest, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "register"}, time.Now())
	if err := n.state.RegisterNode(index, req.Node); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: RegisterNode failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyDeregister(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister"}, time.Now())
	var req structs.DeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeregisterNode(index, req.NodeID); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeregisterNode failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyStatusUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_status_update"}, time.Now())
	var req structs.UpdateStatusRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateNodeStatus(index, req.NodeID, req.Status); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateNodeStatus failed: %v", err)
		return err
	}
	return nil
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
	return
}
