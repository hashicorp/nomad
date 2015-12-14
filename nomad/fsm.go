package nomad

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

const (
	// timeTableGranularity is the granularity of index to time tracking
	timeTableGranularity = 5 * time.Minute

	// timeTableLimit is the maximum limit of our tracking
	timeTableLimit = 72 * time.Hour
)

// SnapshotType is prefixed to a record in the FSM snapshot
// so that we can determine the type for restore
type SnapshotType byte

const (
	NodeSnapshot SnapshotType = iota
	JobSnapshot
	IndexSnapshot
	EvalSnapshot
	AllocSnapshot
	TimeTableSnapshot
	PeriodicLaunchSnapshot
)

// nomadFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type nomadFSM struct {
	evalBroker     *EvalBroker
	periodicRunner PeriodicRunner
	logOutput      io.Writer
	logger         *log.Logger
	state          *state.StateStore
	timetable      *TimeTable
}

// nomadSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type nomadSnapshot struct {
	snap      *state.StateSnapshot
	timetable *TimeTable
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct {
}

// NewFSMPath is used to construct a new FSM with a blank state
func NewFSM(evalBroker *EvalBroker, periodic PeriodicRunner, logOutput io.Writer) (*nomadFSM, error) {
	// Create a state store
	state, err := state.NewStateStore(logOutput)
	if err != nil {
		return nil, err
	}

	fsm := &nomadFSM{
		evalBroker:     evalBroker,
		periodicRunner: periodic,
		logOutput:      logOutput,
		logger:         log.New(logOutput, "", log.LstdFlags),
		state:          state,
		timetable:      NewTimeTable(timeTableGranularity, timeTableLimit),
	}
	return fsm, nil
}

// Close is used to cleanup resources associated with the FSM
func (n *nomadFSM) Close() error {
	return nil
}

// State is used to return a handle to the current state
func (n *nomadFSM) State() *state.StateStore {
	return n.state
}

// TimeTable returns the time table of transactions
func (n *nomadFSM) TimeTable() *TimeTable {
	return n.timetable
}

func (n *nomadFSM) Apply(log *raft.Log) interface{} {
	buf := log.Data
	msgType := structs.MessageType(buf[0])

	// Witness this write
	n.timetable.Witness(log.Index, time.Now().UTC())

	// Check if this message type should be ignored when unknown. This is
	// used so that new commands can be added with developer control if older
	// versions can safely ignore the command, or if they should crash.
	ignoreUnknown := false
	if msgType&structs.IgnoreUnknownTypeFlag == structs.IgnoreUnknownTypeFlag {
		msgType &= ^structs.IgnoreUnknownTypeFlag
		ignoreUnknown = true
	}

	switch msgType {
	case structs.NodeRegisterRequestType:
		return n.applyUpsertNode(buf[1:], log.Index)
	case structs.NodeDeregisterRequestType:
		return n.applyDeregisterNode(buf[1:], log.Index)
	case structs.NodeUpdateStatusRequestType:
		return n.applyStatusUpdate(buf[1:], log.Index)
	case structs.NodeUpdateDrainRequestType:
		return n.applyDrainUpdate(buf[1:], log.Index)
	case structs.JobRegisterRequestType:
		return n.applyUpsertJob(buf[1:], log.Index)
	case structs.JobDeregisterRequestType:
		return n.applyDeregisterJob(buf[1:], log.Index)
	case structs.EvalUpdateRequestType:
		return n.applyUpdateEval(buf[1:], log.Index)
	case structs.EvalDeleteRequestType:
		return n.applyDeleteEval(buf[1:], log.Index)
	case structs.AllocUpdateRequestType:
		return n.applyAllocUpdate(buf[1:], log.Index)
	case structs.AllocClientUpdateRequestType:
		return n.applyAllocClientUpdate(buf[1:], log.Index)
	default:
		if ignoreUnknown {
			n.logger.Printf("[WARN] nomad.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
			return nil
		} else {
			panic(fmt.Errorf("failed to apply request: %#v", buf))
		}
	}
}

func (n *nomadFSM) applyUpsertNode(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "register_node"}, time.Now())
	var req structs.NodeRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertNode(index, req.Node); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertNode failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyDeregisterNode(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_node"}, time.Now())
	var req structs.NodeDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNode(index, req.NodeID); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteNode failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyStatusUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_status_update"}, time.Now())
	var req structs.NodeUpdateStatusRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateNodeStatus(index, req.NodeID, req.Status); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateNodeStatus failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyDrainUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_drain_update"}, time.Now())
	var req structs.NodeUpdateDrainRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateNodeDrain(index, req.NodeID, req.Drain); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateNodeDrain failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyUpsertJob(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "register_job"}, time.Now())
	var req structs.JobRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertJob(index, req.Job); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertJob failed: %v", err)
		return err
	}

	// If it is periodic, insert it into the periodic runner and record the
	// time it was inserted.
	if req.Job.IsPeriodic() {
		if err := n.periodicRunner.Add(req.Job); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: PeriodicRunner.Add failed: %v", err)
			return err
		}

		// Record the insertion time as a launch.
		launch := &structs.PeriodicLaunch{req.Job.ID, time.Now()}
		if err := n.state.UpsertPeriodicLaunch(index, launch); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: UpsertPeriodicLaunch failed: %v", err)
			return err
		}
	}

	// Check if the parent job is periodic and mark the launch time.
	parentID := req.Job.ParentID
	if parentID != "" {
		parent, err := n.state.JobByID(parentID)
		if err != nil {
			n.logger.Printf("[ERR] nomad.fsm: JobByID(%v) lookup for parent failed: %v", parentID, err)
			return err
		} else if parent == nil {
			// The parent has been deregistered.
			return nil
		}

		if parent.IsPeriodic() {
			t, err := n.periodicRunner.LaunchTime(req.Job.ID)
			if err != nil {
				n.logger.Printf("[ERR] nomad.fsm: LaunchTime(%v) failed: %v", req.Job.ID, err)
				return err
			}
			launch := &structs.PeriodicLaunch{parentID, t}
			if err := n.state.UpsertPeriodicLaunch(index, launch); err != nil {
				n.logger.Printf("[ERR] nomad.fsm: UpsertPeriodicLaunch failed: %v", err)
				return err
			}
		}
	}

	return nil
}

func (n *nomadFSM) applyDeregisterJob(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_job"}, time.Now())
	var req structs.JobDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	job, err := n.state.JobByID(req.JobID)
	if err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteJob failed: %v", err)
		return err
	}

	if err := n.state.DeleteJob(index, req.JobID); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteJob failed: %v", err)
		return err
	}

	if job.IsPeriodic() {
		if err := n.periodicRunner.Remove(req.JobID); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: PeriodicRunner.Remove failed: %v", err)
			return err
		}

		if err := n.state.DeletePeriodicLaunch(index, req.JobID); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: DeletePeriodicLaunch failed: %v", err)
			return err
		}
	}

	return nil
}

func (n *nomadFSM) applyUpdateEval(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "update_eval"}, time.Now())
	var req structs.EvalUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertEvals(index, req.Evals); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertEvals failed: %v", err)
		return err
	}

	for _, eval := range req.Evals {
		if eval.ShouldEnqueue() {
			if err := n.evalBroker.Enqueue(eval); err != nil {
				n.logger.Printf("[ERR] nomad.fsm: failed to enqueue evaluation %s: %v", eval.ID, err)
				return err
			}
		}
	}
	return nil
}

func (n *nomadFSM) applyDeleteEval(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "delete_eval"}, time.Now())
	var req structs.EvalDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteEval(index, req.Evals, req.Allocs); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteEval failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyAllocUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "alloc_update"}, time.Now())
	var req structs.AllocUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertAllocs(index, req.Alloc); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertAllocs failed: %v", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyAllocClientUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "alloc_client_update"}, time.Now())
	var req structs.AllocUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	if len(req.Alloc) == 0 {
		return nil
	}

	if err := n.state.UpdateAllocFromClient(index, req.Alloc[0]); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateAllocFromClient failed: %v", err)
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

	ns := &nomadSnapshot{
		snap:      snap,
		timetable: n.timetable,
	}
	return ns, nil
}

func (n *nomadFSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a new state store
	newState, err := state.NewStateStore(n.logOutput)
	if err != nil {
		return err
	}
	n.state = newState

	// Start the state restore
	restore, err := newState.Restore()
	if err != nil {
		return err
	}
	defer restore.Abort()

	// Create a decoder
	dec := codec.NewDecoder(old, structs.MsgpackHandle)

	// Read in the header
	var header snapshotHeader
	if err := dec.Decode(&header); err != nil {
		return err
	}

	// Populate the new state
	msgType := make([]byte, 1)
	for {
		// Read the message type
		_, err := old.Read(msgType)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Decode
		switch SnapshotType(msgType[0]) {
		case TimeTableSnapshot:
			if err := n.timetable.Deserialize(dec); err != nil {
				return fmt.Errorf("time table deserialize failed: %v", err)
			}

		case NodeSnapshot:
			node := new(structs.Node)
			if err := dec.Decode(node); err != nil {
				return err
			}
			if err := restore.NodeRestore(node); err != nil {
				return err
			}

		case JobSnapshot:
			job := new(structs.Job)
			if err := dec.Decode(job); err != nil {
				return err
			}
			if err := restore.JobRestore(job); err != nil {
				return err
			}

		case EvalSnapshot:
			eval := new(structs.Evaluation)
			if err := dec.Decode(eval); err != nil {
				return err
			}
			if err := restore.EvalRestore(eval); err != nil {
				return err
			}

		case AllocSnapshot:
			alloc := new(structs.Allocation)
			if err := dec.Decode(alloc); err != nil {
				return err
			}
			if err := restore.AllocRestore(alloc); err != nil {
				return err
			}

		case IndexSnapshot:
			idx := new(state.IndexEntry)
			if err := dec.Decode(idx); err != nil {
				return err
			}
			if err := restore.IndexRestore(idx); err != nil {
				return err
			}

		case PeriodicLaunchSnapshot:
			launch := new(structs.PeriodicLaunch)
			if err := dec.Decode(launch); err != nil {
				return err
			}
			if err := restore.PeriodicLaunchRestore(launch); err != nil {
				return err
			}

		default:
			return fmt.Errorf("Unrecognized snapshot type: %v", msgType)
		}
	}

	// Commit the state restore
	restore.Commit()
	return nil
}

func (s *nomadSnapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "persist"}, time.Now())
	// Register the nodes
	encoder := codec.NewEncoder(sink, structs.MsgpackHandle)

	// Write the header
	header := snapshotHeader{}
	if err := encoder.Encode(&header); err != nil {
		sink.Cancel()
		return err
	}

	// Write the time table
	sink.Write([]byte{byte(TimeTableSnapshot)})
	if err := s.timetable.Serialize(encoder); err != nil {
		sink.Cancel()
		return err
	}

	// Write all the data out
	if err := s.persistIndexes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistNodes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistJobs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistEvals(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistAllocs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistPeriodicLaunches(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *nomadSnapshot) persistIndexes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the indexes
	iter, err := s.snap.Indexes()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		idx := raw.(*state.IndexEntry)

		// Write out a node registration
		sink.Write([]byte{byte(IndexSnapshot)})
		if err := encoder.Encode(idx); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistNodes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the nodes
	nodes, err := s.snap.Nodes()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := nodes.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		node := raw.(*structs.Node)

		// Write out a node registration
		sink.Write([]byte{byte(NodeSnapshot)})
		if err := encoder.Encode(node); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistJobs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	jobs, err := s.snap.Jobs()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := jobs.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		job := raw.(*structs.Job)

		// Write out a job registration
		sink.Write([]byte{byte(JobSnapshot)})
		if err := encoder.Encode(job); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistEvals(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the evaluations
	evals, err := s.snap.Evals()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := evals.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		eval := raw.(*structs.Evaluation)

		// Write out the evaluation
		sink.Write([]byte{byte(EvalSnapshot)})
		if err := encoder.Encode(eval); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistAllocs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the allocations
	allocs, err := s.snap.Allocs()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := allocs.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		alloc := raw.(*structs.Allocation)

		// Write out the evaluation
		sink.Write([]byte{byte(AllocSnapshot)})
		if err := encoder.Encode(alloc); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistPeriodicLaunches(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	launches, err := s.snap.PeriodicLaunches()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := launches.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		launch := raw.(*structs.PeriodicLaunch)

		// Write out a job registration
		sink.Write([]byte{byte(PeriodicLaunchSnapshot)})
		if err := encoder.Encode(launch); err != nil {
			return err
		}
	}
	return nil
}

// Release is a no-op, as we just need to GC the pointer
// to the state store snapshot. There is nothing to explicitly
// cleanup.
func (s *nomadSnapshot) Release() {}
