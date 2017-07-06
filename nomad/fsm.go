package nomad

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
	"github.com/ugorji/go/codec"
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
	JobSummarySnapshot
	VaultAccessorSnapshot
	JobVersionSnapshot
	DeploymentSnapshot
)

// nomadFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type nomadFSM struct {
	evalBroker         *EvalBroker
	blockedEvals       *BlockedEvals
	periodicDispatcher *PeriodicDispatch
	logOutput          io.Writer
	logger             *log.Logger
	state              *state.StateStore
	timetable          *TimeTable

	// stateLock is only used to protect outside callers to State() from
	// racing with Restore(), which is called by Raft (it puts in a totally
	// new state store). Everything internal here is synchronized by the
	// Raft side, so doesn't need to lock this.
	stateLock sync.RWMutex
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
func NewFSM(evalBroker *EvalBroker, periodic *PeriodicDispatch,
	blocked *BlockedEvals, logOutput io.Writer) (*nomadFSM, error) {
	// Create a state store
	state, err := state.NewStateStore(logOutput)
	if err != nil {
		return nil, err
	}

	fsm := &nomadFSM{
		evalBroker:         evalBroker,
		periodicDispatcher: periodic,
		blockedEvals:       blocked,
		logOutput:          logOutput,
		logger:             log.New(logOutput, "", log.LstdFlags),
		state:              state,
		timetable:          NewTimeTable(timeTableGranularity, timeTableLimit),
	}
	return fsm, nil
}

// Close is used to cleanup resources associated with the FSM
func (n *nomadFSM) Close() error {
	return nil
}

// State is used to return a handle to the current state
func (n *nomadFSM) State() *state.StateStore {
	n.stateLock.RLock()
	defer n.stateLock.RUnlock()
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
	case structs.ReconcileJobSummariesRequestType:
		return n.applyReconcileSummaries(buf[1:], log.Index)
	case structs.VaultAccessorRegisterRequestType:
		return n.applyUpsertVaultAccessor(buf[1:], log.Index)
	case structs.VaultAccessorDegisterRequestType:
		return n.applyDeregisterVaultAccessor(buf[1:], log.Index)
	case structs.ApplyPlanResultsRequestType:
		return n.applyPlanResults(buf[1:], log.Index)
	case structs.DeploymentStatusUpdateRequestType:
		return n.applyDeploymentStatusUpdate(buf[1:], log.Index)
	case structs.DeploymentPromoteRequestType:
		return n.applyDeploymentPromotion(buf[1:], log.Index)
	case structs.DeploymentAllocHealthRequestType:
		return n.applyDeploymentAllocHealth(buf[1:], log.Index)
	case structs.DeploymentDeleteRequestType:
		return n.applyDeploymentDelete(buf[1:], log.Index)
	case structs.JobStabilityRequestType:
		return n.applyJobStability(buf[1:], log.Index)
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

	// Unblock evals for the nodes computed node class if it is in a ready
	// state.
	if req.Node.Status == structs.NodeStatusReady {
		n.blockedEvals.Unblock(req.Node.ComputedClass, index)
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

	// Unblock evals for the nodes computed node class if it is in a ready
	// state.
	if req.Status == structs.NodeStatusReady {
		ws := memdb.NewWatchSet()
		node, err := n.state.NodeByID(ws, req.NodeID)
		if err != nil {
			n.logger.Printf("[ERR] nomad.fsm: looking up node %q failed: %v", req.NodeID, err)
			return err

		}
		n.blockedEvals.Unblock(node.ComputedClass, index)
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

	/* Handle upgrade paths:
	 * - Empty maps and slices should be treated as nil to avoid
	 *   un-intended destructive updates in scheduler since we use
	 *   reflect.DeepEqual. Starting Nomad 0.4.1, job submission sanatizes
	 *   the incoming job.
	 * - Migrate from old style upgrade stanza that used only a stagger.
	 */
	req.Job.Canonicalize()

	if err := n.state.UpsertJob(index, req.Job); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertJob failed: %v", err)
		return err
	}

	// We always add the job to the periodic dispatcher because there is the
	// possibility that the periodic spec was removed and then we should stop
	// tracking it.
	if err := n.periodicDispatcher.Add(req.Job); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: periodicDispatcher.Add failed: %v", err)
		return err
	}

	// Create a watch set
	ws := memdb.NewWatchSet()

	// If it is periodic, record the time it was inserted. This is necessary for
	// recovering during leader election. It is possible that from the time it
	// is added to when it was suppose to launch, leader election occurs and the
	// job was not launched. In this case, we use the insertion time to
	// determine if a launch was missed.
	if req.Job.IsPeriodic() {
		prevLaunch, err := n.state.PeriodicLaunchByID(ws, req.Job.ID)
		if err != nil {
			n.logger.Printf("[ERR] nomad.fsm: PeriodicLaunchByID failed: %v", err)
			return err
		}

		// Record the insertion time as a launch. We overload the launch table
		// such that the first entry is the insertion time.
		if prevLaunch == nil {
			launch := &structs.PeriodicLaunch{ID: req.Job.ID, Launch: time.Now()}
			if err := n.state.UpsertPeriodicLaunch(index, launch); err != nil {
				n.logger.Printf("[ERR] nomad.fsm: UpsertPeriodicLaunch failed: %v", err)
				return err
			}
		}
	}

	// Check if the parent job is periodic and mark the launch time.
	parentID := req.Job.ParentID
	if parentID != "" {
		parent, err := n.state.JobByID(ws, parentID)
		if err != nil {
			n.logger.Printf("[ERR] nomad.fsm: JobByID(%v) lookup for parent failed: %v", parentID, err)
			return err
		} else if parent == nil {
			// The parent has been deregistered.
			return nil
		}

		if parent.IsPeriodic() && !parent.IsParameterized() {
			t, err := n.periodicDispatcher.LaunchTime(req.Job.ID)
			if err != nil {
				n.logger.Printf("[ERR] nomad.fsm: LaunchTime(%v) failed: %v", req.Job.ID, err)
				return err
			}

			launch := &structs.PeriodicLaunch{ID: parentID, Launch: t}
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

	// If it is periodic remove it from the dispatcher
	if err := n.periodicDispatcher.Remove(req.JobID); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: periodicDispatcher.Remove failed: %v", err)
		return err
	}

	if req.Purge {
		if err := n.state.DeleteJob(index, req.JobID); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: DeleteJob failed: %v", err)
			return err
		}

		// We always delete from the periodic launch table because it is possible that
		// the job was updated to be non-perioidic, thus checking if it is periodic
		// doesn't ensure we clean it up properly.
		n.state.DeletePeriodicLaunch(index, req.JobID)
	} else {
		// Get the current job and mark it as stopped and re-insert it.
		ws := memdb.NewWatchSet()
		current, err := n.state.JobByID(ws, req.JobID)
		if err != nil {
			n.logger.Printf("[ERR] nomad.fsm: JobByID lookup failed: %v", err)
			return err
		}

		if current == nil {
			return fmt.Errorf("job %q doesn't exist to be deregistered", req.JobID)
		}

		stopped := current.Copy()
		stopped.Stop = true

		if err := n.state.UpsertJob(index, stopped); err != nil {
			n.logger.Printf("[ERR] nomad.fsm: UpsertJob failed: %v", err)
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
			n.evalBroker.Enqueue(eval)
		} else if eval.ShouldBlock() {
			n.blockedEvals.Block(eval)
		} else if eval.Status == structs.EvalStatusComplete &&
			len(eval.FailedTGAllocs) == 0 {
			// If we have a successful evaluation for a node, untrack any
			// blocked evaluation
			n.blockedEvals.Untrack(eval.JobID)
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

	// Attach the job to all the allocations. It is pulled out in the
	// payload to avoid the redundancy of encoding, but should be denormalized
	// prior to being inserted into MemDB.
	structs.DenormalizeAllocationJobs(req.Job, req.Alloc)

	// Calculate the total resources of allocations. It is pulled out in the
	// payload to avoid encoding something that can be computed, but should be
	// denormalized prior to being inserted into MemDB.
	for _, alloc := range req.Alloc {
		if alloc.Resources != nil {
			// COMPAT 0.4.1 -> 0.5
			// Set the shared resources for allocations which don't have them
			if alloc.SharedResources == nil {
				alloc.SharedResources = &structs.Resources{
					DiskMB: alloc.Resources.DiskMB,
				}
			}

			continue
		}

		alloc.Resources = new(structs.Resources)
		for _, task := range alloc.TaskResources {
			alloc.Resources.Add(task)
		}

		// Add the shared resources
		alloc.Resources.Add(alloc.SharedResources)
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

	// Create a watch set
	ws := memdb.NewWatchSet()

	// Updating the allocs with the job id and task group name
	for _, alloc := range req.Alloc {
		if existing, _ := n.state.AllocByID(ws, alloc.ID); existing != nil {
			alloc.JobID = existing.JobID
			alloc.TaskGroup = existing.TaskGroup
		}
	}

	// Update all the client allocations
	if err := n.state.UpdateAllocsFromClient(index, req.Alloc); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateAllocFromClient failed: %v", err)
		return err
	}

	// Unblock evals for the nodes computed node class if the client has
	// finished running an allocation.
	for _, alloc := range req.Alloc {
		if alloc.ClientStatus == structs.AllocClientStatusComplete ||
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			nodeID := alloc.NodeID
			node, err := n.state.NodeByID(ws, nodeID)
			if err != nil || node == nil {
				n.logger.Printf("[ERR] nomad.fsm: looking up node %q failed: %v", nodeID, err)
				return err

			}
			n.blockedEvals.Unblock(node.ComputedClass, index)
		}
	}

	return nil
}

// applyReconcileSummaries reconciles summaries for all the jobs
func (n *nomadFSM) applyReconcileSummaries(buf []byte, index uint64) interface{} {
	if err := n.state.ReconcileJobSummaries(index); err != nil {
		return err
	}
	return n.reconcileQueuedAllocations(index)
}

// applyUpsertVaultAccessor stores the Vault accessors for a given allocation
// and task
func (n *nomadFSM) applyUpsertVaultAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_vault_accessor"}, time.Now())
	var req structs.VaultAccessorsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertVaultAccessor(index, req.Accessors); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertVaultAccessor failed: %v", err)
		return err
	}

	return nil
}

// applyDeregisterVaultAccessor deregisters a set of Vault accessors
func (n *nomadFSM) applyDeregisterVaultAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_vault_accessor"}, time.Now())
	var req structs.VaultAccessorsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteVaultAccessors(index, req.Accessors); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeregisterVaultAccessor failed: %v", err)
		return err
	}

	return nil
}

// applyPlanApply applies the results of a plan application
func (n *nomadFSM) applyPlanResults(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_plan_results"}, time.Now())
	var req structs.ApplyPlanResultsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertPlanResults(index, &req); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: ApplyPlan failed: %v", err)
		return err
	}

	return nil
}

// applyDeploymentStatusUpdate is used to update the status of an existing
// deployment
func (n *nomadFSM) applyDeploymentStatusUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_status_update"}, time.Now())
	var req structs.DeploymentStatusUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentStatus(index, &req); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertDeploymentStatusUpdate failed: %v", err)
		return err
	}

	if req.Eval != nil && req.Eval.ShouldEnqueue() {
		n.evalBroker.Enqueue(req.Eval)
	}

	return nil
}

// applyDeploymentPromotion is used to promote canaries in a deployment
func (n *nomadFSM) applyDeploymentPromotion(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_promotion"}, time.Now())
	var req structs.ApplyDeploymentPromoteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentPromotion(index, &req); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertDeploymentPromotion failed: %v", err)
		return err
	}

	if req.Eval != nil && req.Eval.ShouldEnqueue() {
		n.evalBroker.Enqueue(req.Eval)
	}

	return nil
}

// applyDeploymentAllocHealth is used to set the health of allocations as part
// of a deployment
func (n *nomadFSM) applyDeploymentAllocHealth(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_alloc_health"}, time.Now())
	var req structs.ApplyDeploymentAllocHealthRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentAllocHealth(index, &req); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertDeploymentAllocHealth failed: %v", err)
		return err
	}

	if req.Eval != nil && req.Eval.ShouldEnqueue() {
		n.evalBroker.Enqueue(req.Eval)
	}

	return nil
}

// applyDeploymentDelete is used to delete a set of deployments
func (n *nomadFSM) applyDeploymentDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_delete"}, time.Now())
	var req structs.DeploymentDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteDeployment(index, req.Deployments); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteDeployment failed: %v", err)
		return err
	}

	return nil
}

// applyJobStability is used to set the stability of a job
func (n *nomadFSM) applyJobStability(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_job_stability"}, time.Now())
	var req structs.JobStabilityRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateJobStability(index, req.JobID, req.JobVersion, req.Stable); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpdateJobStability failed: %v", err)
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

			/* Handle upgrade paths:
			 * - Empty maps and slices should be treated as nil to avoid
			 *   un-intended destructive updates in scheduler since we use
			 *   reflect.DeepEqual. Starting Nomad 0.4.1, job submission sanatizes
			 *   the incoming job.
			 * - Migrate from old style upgrade stanza that used only a stagger.
			 */
			job.Canonicalize()

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

		case JobSummarySnapshot:
			summary := new(structs.JobSummary)
			if err := dec.Decode(summary); err != nil {
				return err
			}
			if err := restore.JobSummaryRestore(summary); err != nil {
				return err
			}

		case VaultAccessorSnapshot:
			accessor := new(structs.VaultAccessor)
			if err := dec.Decode(accessor); err != nil {
				return err
			}
			if err := restore.VaultAccessorRestore(accessor); err != nil {
				return err
			}

		case JobVersionSnapshot:
			version := new(structs.Job)
			if err := dec.Decode(version); err != nil {
				return err
			}
			if err := restore.JobVersionRestore(version); err != nil {
				return err
			}

		case DeploymentSnapshot:
			deployment := new(structs.Deployment)
			if err := dec.Decode(deployment); err != nil {
				return err
			}
			if err := restore.DeploymentRestore(deployment); err != nil {
				return err
			}

		default:
			return fmt.Errorf("Unrecognized snapshot type: %v", msgType)
		}
	}

	restore.Commit()

	// Create Job Summaries
	// COMPAT 0.4 -> 0.4.1
	// We can remove this in 0.5. This exists so that the server creates job
	// summaries if they were not present previously. When users upgrade to 0.5
	// from 0.4.1, the snapshot will contain job summaries so it will be safe to
	// remove this block.
	index, err := newState.Index("job_summary")
	if err != nil {
		return fmt.Errorf("couldn't fetch index of job summary table: %v", err)
	}

	// If the index is 0 that means there is no job summary in the snapshot so
	// we will have to create them
	if index == 0 {
		// query the latest index
		latestIndex, err := newState.LatestIndex()
		if err != nil {
			return fmt.Errorf("unable to query latest index: %v", index)
		}
		if err := newState.ReconcileJobSummaries(latestIndex); err != nil {
			return fmt.Errorf("error reconciling summaries: %v", err)
		}
	}

	// External code might be calling State(), so we need to synchronize
	// here to make sure we swap in the new state store atomically.
	n.stateLock.Lock()
	stateOld := n.state
	n.state = newState
	n.stateLock.Unlock()

	// Signal that the old state store has been abandoned. This is required
	// because we don't operate on it any more, we just throw it away, so
	// blocking queries won't see any changes and need to be woken up.
	stateOld.Abandon()

	return nil
}

// reconcileSummaries re-calculates the queued allocations for every job that we
// created a Job Summary during the snap shot restore
func (n *nomadFSM) reconcileQueuedAllocations(index uint64) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	iter, err := n.state.Jobs(ws)
	if err != nil {
		return err
	}

	snap, err := n.state.Snapshot()
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %v", err)
	}

	// Invoking the scheduler for every job so that we can populate the number
	// of queued allocations for every job
	for {
		rawJob := iter.Next()
		if rawJob == nil {
			break
		}
		job := rawJob.(*structs.Job)
		planner := &scheduler.Harness{
			State: &snap.StateStore,
		}
		// Create an eval and mark it as requiring annotations and insert that as well
		eval := &structs.Evaluation{
			ID:             structs.GenerateUUID(),
			Priority:       job.Priority,
			Type:           job.Type,
			TriggeredBy:    structs.EvalTriggerJobRegister,
			JobID:          job.ID,
			JobModifyIndex: job.JobModifyIndex + 1,
			Status:         structs.EvalStatusPending,
			AnnotatePlan:   true,
		}

		// Create the scheduler and run it
		sched, err := scheduler.NewScheduler(eval.Type, n.logger, snap, planner)
		if err != nil {
			return err
		}

		if err := sched.Process(eval); err != nil {
			return err
		}

		// Get the job summary from the fsm state store
		originalSummary, err := n.state.JobSummaryByID(ws, job.ID)
		if err != nil {
			return err
		}
		summary := originalSummary.Copy()

		// Add the allocations scheduler has made to queued since these
		// allocations are never getting placed until the scheduler is invoked
		// with a real planner
		if l := len(planner.Plans); l != 1 {
			return fmt.Errorf("unexpected number of plans during restore %d. Please file an issue including the logs", l)
		}
		for _, allocations := range planner.Plans[0].NodeAllocation {
			for _, allocation := range allocations {
				tgSummary, ok := summary.Summary[allocation.TaskGroup]
				if !ok {
					return fmt.Errorf("task group %q not found while updating queued count", allocation.TaskGroup)
				}
				tgSummary.Queued += 1
				summary.Summary[allocation.TaskGroup] = tgSummary
			}
		}

		// Add the queued allocations attached to the evaluation to the queued
		// counter of the job summary
		if l := len(planner.Evals); l != 1 {
			return fmt.Errorf("unexpected number of evals during restore %d. Please file an issue including the logs", l)
		}
		for tg, queued := range planner.Evals[0].QueuedAllocations {
			tgSummary, ok := summary.Summary[tg]
			if !ok {
				return fmt.Errorf("task group %q not found while updating queued count", tg)
			}

			// We add instead of setting here because we want to take into
			// consideration what the scheduler with a mock planner thinks it
			// placed. Those should be counted as queued as well
			tgSummary.Queued += queued
			summary.Summary[tg] = tgSummary
		}

		if !reflect.DeepEqual(summary, originalSummary) {
			summary.ModifyIndex = index
			if err := n.state.UpsertJobSummary(index, summary); err != nil {
				return err
			}
		}
	}
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
	if err := s.persistJobSummaries(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistVaultAccessors(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistJobVersions(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistDeployments(sink, encoder); err != nil {
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
	ws := memdb.NewWatchSet()
	nodes, err := s.snap.Nodes(ws)
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
	ws := memdb.NewWatchSet()
	jobs, err := s.snap.Jobs(ws)
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
	ws := memdb.NewWatchSet()
	evals, err := s.snap.Evals(ws)
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
	ws := memdb.NewWatchSet()
	allocs, err := s.snap.Allocs(ws)
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
	ws := memdb.NewWatchSet()
	launches, err := s.snap.PeriodicLaunches(ws)
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

func (s *nomadSnapshot) persistJobSummaries(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	summaries, err := s.snap.JobSummaries(ws)
	if err != nil {
		return err
	}

	for {
		raw := summaries.Next()
		if raw == nil {
			break
		}

		jobSummary := raw.(*structs.JobSummary)

		sink.Write([]byte{byte(JobSummarySnapshot)})
		if err := encoder.Encode(jobSummary); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistVaultAccessors(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	accessors, err := s.snap.VaultAccessors(ws)
	if err != nil {
		return err
	}

	for {
		raw := accessors.Next()
		if raw == nil {
			break
		}

		accessor := raw.(*structs.VaultAccessor)

		sink.Write([]byte{byte(VaultAccessorSnapshot)})
		if err := encoder.Encode(accessor); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistJobVersions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	versions, err := s.snap.JobVersions(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := versions.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		job := raw.(*structs.Job)

		// Write out a job registration
		sink.Write([]byte{byte(JobVersionSnapshot)})
		if err := encoder.Encode(job); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistDeployments(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	deployments, err := s.snap.Deployments(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := deployments.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		deployment := raw.(*structs.Deployment)

		// Write out a job registration
		sink.Write([]byte{byte(DeploymentSnapshot)})
		if err := encoder.Encode(deployment); err != nil {
			return err
		}
	}
	return nil
}

// Release is a no-op, as we just need to GC the pointer
// to the state store snapshot. There is nothing to explicitly
// cleanup.
func (s *nomadSnapshot) Release() {}
