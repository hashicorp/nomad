package nomad

import (
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
	"github.com/pkg/errors"
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
	ACLPolicySnapshot
	ACLTokenSnapshot
	SchedulerConfigSnapshot
	ClusterMetadataSnapshot
	ServiceIdentityTokenAccessorSnapshot
	ScalingPolicySnapshot
	CSIPluginSnapshot
	CSIVolumeSnapshot
	ScalingEventsSnapshot
)

// LogApplier is the definition of a function that can apply a Raft log
type LogApplier func(buf []byte, index uint64) interface{}

// LogAppliers is a mapping of the Raft MessageType to the appropriate log
// applier
type LogAppliers map[structs.MessageType]LogApplier

// SnapshotRestorer is the definition of a function that can apply a Raft log
type SnapshotRestorer func(restore *state.StateRestore, dec *codec.Decoder) error

// SnapshotRestorers is a mapping of the SnapshotType to the appropriate
// snapshot restorer.
type SnapshotRestorers map[SnapshotType]SnapshotRestorer

// nomadFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type nomadFSM struct {
	evalBroker         *EvalBroker
	blockedEvals       *BlockedEvals
	periodicDispatcher *PeriodicDispatch
	logger             log.Logger
	state              *state.StateStore
	timetable          *TimeTable

	// config is the FSM config
	config *FSMConfig

	// enterpriseAppliers holds the set of enterprise only LogAppliers
	enterpriseAppliers LogAppliers

	// enterpriseRestorers holds the set of enterprise only snapshot restorers
	enterpriseRestorers SnapshotRestorers

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

// FSMConfig is used to configure the FSM
type FSMConfig struct {
	// EvalBroker is the evaluation broker evaluations should be added to
	EvalBroker *EvalBroker

	// Periodic is the periodic job dispatcher that periodic jobs should be
	// added/removed from
	Periodic *PeriodicDispatch

	// BlockedEvals is the blocked eval tracker that blocked evaluations should
	// be added to.
	Blocked *BlockedEvals

	// Logger is the logger used by the FSM
	Logger log.Logger

	// Region is the region of the server embedding the FSM
	Region string
}

// NewFSMPath is used to construct a new FSM with a blank state
func NewFSM(config *FSMConfig) (*nomadFSM, error) {
	// Create a state store
	sconfig := &state.StateStoreConfig{
		Logger: config.Logger,
		Region: config.Region,
	}
	state, err := state.NewStateStore(sconfig)
	if err != nil {
		return nil, err
	}

	fsm := &nomadFSM{
		evalBroker:          config.EvalBroker,
		periodicDispatcher:  config.Periodic,
		blockedEvals:        config.Blocked,
		logger:              config.Logger.Named("fsm"),
		config:              config,
		state:               state,
		timetable:           NewTimeTable(timeTableGranularity, timeTableLimit),
		enterpriseAppliers:  make(map[structs.MessageType]LogApplier, 8),
		enterpriseRestorers: make(map[SnapshotType]SnapshotRestorer, 8),
	}

	// Register all the log applier functions
	fsm.registerLogAppliers()

	// Register all the snapshot restorer functions
	fsm.registerSnapshotRestorers()

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
	case structs.VaultAccessorDeregisterRequestType:
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
	case structs.ACLPolicyUpsertRequestType:
		return n.applyACLPolicyUpsert(buf[1:], log.Index)
	case structs.ACLPolicyDeleteRequestType:
		return n.applyACLPolicyDelete(buf[1:], log.Index)
	case structs.ACLTokenUpsertRequestType:
		return n.applyACLTokenUpsert(buf[1:], log.Index)
	case structs.ACLTokenDeleteRequestType:
		return n.applyACLTokenDelete(buf[1:], log.Index)
	case structs.ACLTokenBootstrapRequestType:
		return n.applyACLTokenBootstrap(buf[1:], log.Index)
	case structs.AutopilotRequestType:
		return n.applyAutopilotUpdate(buf[1:], log.Index)
	case structs.UpsertNodeEventsType:
		return n.applyUpsertNodeEvent(buf[1:], log.Index)
	case structs.JobBatchDeregisterRequestType:
		return n.applyBatchDeregisterJob(buf[1:], log.Index)
	case structs.AllocUpdateDesiredTransitionRequestType:
		return n.applyAllocUpdateDesiredTransition(buf[1:], log.Index)
	case structs.NodeUpdateEligibilityRequestType:
		return n.applyNodeEligibilityUpdate(buf[1:], log.Index)
	case structs.BatchNodeUpdateDrainRequestType:
		return n.applyBatchDrainUpdate(buf[1:], log.Index)
	case structs.SchedulerConfigRequestType:
		return n.applySchedulerConfigUpdate(buf[1:], log.Index)
	case structs.NodeBatchDeregisterRequestType:
		return n.applyDeregisterNodeBatch(buf[1:], log.Index)
	case structs.ClusterMetadataRequestType:
		return n.applyClusterMetadata(buf[1:], log.Index)
	case structs.ServiceIdentityAccessorRegisterRequestType:
		return n.applyUpsertSIAccessor(buf[1:], log.Index)
	case structs.ServiceIdentityAccessorDeregisterRequestType:
		return n.applyDeregisterSIAccessor(buf[1:], log.Index)
	case structs.CSIVolumeRegisterRequestType:
		return n.applyCSIVolumeRegister(buf[1:], log.Index)
	case structs.CSIVolumeDeregisterRequestType:
		return n.applyCSIVolumeDeregister(buf[1:], log.Index)
	case structs.CSIVolumeClaimRequestType:
		return n.applyCSIVolumeClaim(buf[1:], log.Index)
	case structs.ScalingEventRegisterRequestType:
		return n.applyUpsertScalingEvent(buf[1:], log.Index)
	}

	// Check enterprise only message types.
	if applier, ok := n.enterpriseAppliers[msgType]; ok {
		return applier(buf[1:], log.Index)
	}

	// We didn't match anything, either panic or ignore
	if ignoreUnknown {
		n.logger.Warn("ignoring unknown message type, upgrade to newer version", "msg_type", msgType)
		return nil
	}

	panic(fmt.Errorf("failed to apply request: %#v", buf))
}

func (n *nomadFSM) applyClusterMetadata(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "cluster_meta"}, time.Now())

	var req structs.ClusterMetadata
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.ClusterSetMetadata(index, &req); err != nil {
		n.logger.Error("ClusterSetMetadata failed", "error", err)
		return err
	}

	n.logger.Trace("ClusterSetMetadata", "cluster_id", req.ClusterID, "create_time", req.CreateTime)

	return nil
}

func (n *nomadFSM) applyUpsertNode(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "register_node"}, time.Now())
	var req structs.NodeRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Handle upgrade paths
	req.Node.Canonicalize()

	if err := n.state.UpsertNode(index, req.Node); err != nil {
		n.logger.Error("UpsertNode failed", "error", err)
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

	if err := n.state.DeleteNode(index, []string{req.NodeID}); err != nil {
		n.logger.Error("DeleteNode failed", "error", err)
		return err
	}

	return nil
}

func (n *nomadFSM) applyDeregisterNodeBatch(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_deregister_node"}, time.Now())
	var req structs.NodeBatchDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNode(index, req.NodeIDs); err != nil {
		n.logger.Error("DeleteNode failed", "error", err)
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

	if err := n.state.UpdateNodeStatus(index, req.NodeID, req.Status, req.UpdatedAt, req.NodeEvent); err != nil {
		n.logger.Error("UpdateNodeStatus failed", "error", err)
		return err
	}

	// Unblock evals for the nodes computed node class if it is in a ready
	// state.
	if req.Status == structs.NodeStatusReady {
		ws := memdb.NewWatchSet()
		node, err := n.state.NodeByID(ws, req.NodeID)
		if err != nil {
			n.logger.Error("looking up node failed", "node_id", req.NodeID, "error", err)
			return err

		}
		n.blockedEvals.Unblock(node.ComputedClass, index)
		n.blockedEvals.UnblockNode(req.NodeID, index)
	}

	return nil
}

func (n *nomadFSM) applyDrainUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_drain_update"}, time.Now())
	var req structs.NodeUpdateDrainRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// COMPAT Remove in version 0.10
	// As part of Nomad 0.8 we have deprecated the drain boolean in favor of a
	// drain strategy but we need to handle the upgrade path where the Raft log
	// contains drain updates with just the drain boolean being manipulated.
	if req.Drain && req.DrainStrategy == nil {
		// Mark the drain strategy as a force to imitate the old style drain
		// functionality.
		req.DrainStrategy = &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: -1 * time.Second,
			},
		}
	}

	if err := n.state.UpdateNodeDrain(index, req.NodeID, req.DrainStrategy, req.MarkEligible, req.UpdatedAt, req.NodeEvent); err != nil {
		n.logger.Error("UpdateNodeDrain failed", "error", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyBatchDrainUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_node_drain_update"}, time.Now())
	var req structs.BatchNodeUpdateDrainRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.BatchUpdateNodeDrain(index, req.UpdatedAt, req.Updates, req.NodeEvents); err != nil {
		n.logger.Error("BatchUpdateNodeDrain failed", "error", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyNodeEligibilityUpdate(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_eligibility_update"}, time.Now())
	var req structs.NodeUpdateEligibilityRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Lookup the existing node
	node, err := n.state.NodeByID(nil, req.NodeID)
	if err != nil {
		n.logger.Error("UpdateNodeEligibility failed to lookup node", "node_id", req.NodeID, "error", err)
		return err
	}

	if err := n.state.UpdateNodeEligibility(index, req.NodeID, req.Eligibility, req.UpdatedAt, req.NodeEvent); err != nil {
		n.logger.Error("UpdateNodeEligibility failed", "error", err)
		return err
	}

	// Unblock evals for the nodes computed node class if it is in a ready
	// state.
	if node != nil && node.SchedulingEligibility == structs.NodeSchedulingIneligible &&
		req.Eligibility == structs.NodeSchedulingEligible {
		n.blockedEvals.Unblock(node.ComputedClass, index)
		n.blockedEvals.UnblockNode(req.NodeID, index)
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
	 *   reflect.DeepEqual. Starting Nomad 0.4.1, job submission sanitizes
	 *   the incoming job.
	 * - Migrate from old style upgrade stanza that used only a stagger.
	 */
	req.Job.Canonicalize()

	if err := n.state.UpsertJob(index, req.Job); err != nil {
		n.logger.Error("UpsertJob failed", "error", err)
		return err
	}

	// We always add the job to the periodic dispatcher because there is the
	// possibility that the periodic spec was removed and then we should stop
	// tracking it.
	if err := n.periodicDispatcher.Add(req.Job); err != nil {
		n.logger.Error("periodicDispatcher.Add failed", "error", err)
		return fmt.Errorf("failed adding job to periodic dispatcher: %v", err)
	}

	// Create a watch set
	ws := memdb.NewWatchSet()

	// If it is an active periodic job, record the time it was inserted. This is
	// necessary for recovering during leader election. It is possible that from
	// the time it is added to when it was suppose to launch, leader election
	// occurs and the job was not launched. In this case, we use the insertion
	// time to determine if a launch was missed.
	if req.Job.IsPeriodicActive() {
		prevLaunch, err := n.state.PeriodicLaunchByID(ws, req.Namespace, req.Job.ID)
		if err != nil {
			n.logger.Error("PeriodicLaunchByID failed", "error", err)
			return err
		}

		// Record the insertion time as a launch. We overload the launch table
		// such that the first entry is the insertion time.
		if prevLaunch == nil {
			launch := &structs.PeriodicLaunch{
				ID:        req.Job.ID,
				Namespace: req.Namespace,
				Launch:    time.Now(),
			}
			if err := n.state.UpsertPeriodicLaunch(index, launch); err != nil {
				n.logger.Error("UpsertPeriodicLaunch failed", "error", err)
				return err
			}
		}
	}

	// Check if the parent job is periodic and mark the launch time.
	parentID := req.Job.ParentID
	if parentID != "" {
		parent, err := n.state.JobByID(ws, req.Namespace, parentID)
		if err != nil {
			n.logger.Error("JobByID lookup for parent failed", "parent_id", parentID, "namespace", req.Namespace, "error", err)
			return err
		} else if parent == nil {
			// The parent has been deregistered.
			return nil
		}

		if parent.IsPeriodic() && !parent.IsParameterized() {
			t, err := n.periodicDispatcher.LaunchTime(req.Job.ID)
			if err != nil {
				n.logger.Error("LaunchTime failed", "job", req.Job.NamespacedID(), "error", err)
				return err
			}

			launch := &structs.PeriodicLaunch{
				ID:        parentID,
				Namespace: req.Namespace,
				Launch:    t,
			}
			if err := n.state.UpsertPeriodicLaunch(index, launch); err != nil {
				n.logger.Error("UpsertPeriodicLaunch failed", "error", err)
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

	return n.state.WithWriteTransaction(func(tx state.Txn) error {
		if err := n.handleJobDeregister(index, req.JobID, req.Namespace, req.Purge, tx); err != nil {
			n.logger.Error("deregistering job failed", "error", err)
			return err
		}

		return nil
	})
}

func (n *nomadFSM) applyBatchDeregisterJob(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_deregister_job"}, time.Now())
	var req structs.JobBatchDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Perform all store updates atomically to ensure a consistent view for store readers.
	// A partial update may increment the snapshot index, allowing eval brokers to process
	// evals for jobs whose deregistering didn't get committed yet.
	err := n.state.WithWriteTransaction(func(tx state.Txn) error {
		for jobNS, options := range req.Jobs {
			if err := n.handleJobDeregister(index, jobNS.ID, jobNS.Namespace, options.Purge, tx); err != nil {
				n.logger.Error("deregistering job failed", "job", jobNS, "error", err)
				return err
			}
		}

		if err := n.state.UpsertEvalsTxn(index, req.Evals, tx); err != nil {
			n.logger.Error("UpsertEvals failed", "error", err)
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// perform the side effects outside the transactions
	n.handleUpsertedEvals(req.Evals)
	return nil
}

// handleJobDeregister is used to deregister a job.
func (n *nomadFSM) handleJobDeregister(index uint64, jobID, namespace string, purge bool, tx state.Txn) error {
	// If it is periodic remove it from the dispatcher
	if err := n.periodicDispatcher.Remove(namespace, jobID); err != nil {
		n.logger.Error("periodicDispatcher.Remove failed", "error", err)
		return err
	}

	if purge {
		if err := n.state.DeleteJobTxn(index, namespace, jobID, tx); err != nil {
			n.logger.Error("DeleteJob failed", "error", err)
			return err
		}

		// We always delete from the periodic launch table because it is possible that
		// the job was updated to be non-periodic, thus checking if it is periodic
		// doesn't ensure we clean it up properly.
		n.state.DeletePeriodicLaunchTxn(index, namespace, jobID, tx)
	} else {
		// Get the current job and mark it as stopped and re-insert it.
		ws := memdb.NewWatchSet()
		current, err := n.state.JobByIDTxn(ws, namespace, jobID, tx)
		if err != nil {
			n.logger.Error("JobByID lookup failed", "error", err)
			return err
		}

		if current == nil {
			return fmt.Errorf("job %q in namespace %q doesn't exist to be deregistered", jobID, namespace)
		}

		stopped := current.Copy()
		stopped.Stop = true

		if err := n.state.UpsertJobTxn(index, stopped, tx); err != nil {
			n.logger.Error("UpsertJob failed", "error", err)
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
	return n.upsertEvals(index, req.Evals)
}

func (n *nomadFSM) upsertEvals(index uint64, evals []*structs.Evaluation) error {
	if err := n.state.UpsertEvals(index, evals); err != nil {
		n.logger.Error("UpsertEvals failed", "error", err)
		return err
	}

	n.handleUpsertedEvals(evals)
	return nil
}

// handleUpsertingEval is a helper for taking action after upserting
// evaluations.
func (n *nomadFSM) handleUpsertedEvals(evals []*structs.Evaluation) {
	for _, eval := range evals {
		n.handleUpsertedEval(eval)
	}
}

// handleUpsertingEval is a helper for taking action after upserting an eval.
func (n *nomadFSM) handleUpsertedEval(eval *structs.Evaluation) {
	if eval == nil {
		return
	}

	if eval.ShouldEnqueue() {
		n.evalBroker.Enqueue(eval)
	} else if eval.ShouldBlock() {
		n.blockedEvals.Block(eval)
	} else if eval.Status == structs.EvalStatusComplete &&
		len(eval.FailedTGAllocs) == 0 {
		// If we have a successful evaluation for a node, untrack any
		// blocked evaluation
		n.blockedEvals.Untrack(eval.JobID, eval.Namespace)
	}
}

func (n *nomadFSM) applyDeleteEval(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "delete_eval"}, time.Now())
	var req structs.EvalDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteEval(index, req.Evals, req.Allocs); err != nil {
		n.logger.Error("DeleteEval failed", "error", err)
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

	for _, alloc := range req.Alloc {
		// COMPAT(0.11): Remove in 0.11
		// Calculate the total resources of allocations. It is pulled out in the
		// payload to avoid encoding something that can be computed, but should be
		// denormalized prior to being inserted into MemDB.
		if alloc.Resources == nil {
			alloc.Resources = new(structs.Resources)
			for _, task := range alloc.TaskResources {
				alloc.Resources.Add(task)
			}

			// Add the shared resources
			alloc.Resources.Add(alloc.SharedResources)
		}

		// Handle upgrade path
		alloc.Canonicalize()
	}

	if err := n.state.UpsertAllocs(index, req.Alloc); err != nil {
		n.logger.Error("UpsertAllocs failed", "error", err)
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
		n.logger.Error("UpdateAllocFromClient failed", "error", err)
		return err
	}

	// Update any evals
	if len(req.Evals) > 0 {
		if err := n.upsertEvals(index, req.Evals); err != nil {
			n.logger.Error("applyAllocClientUpdate failed to update evaluations", "error", err)
			return err
		}
	}

	// Unblock evals for the nodes computed node class if the client has
	// finished running an allocation.
	for _, alloc := range req.Alloc {
		if alloc.ClientStatus == structs.AllocClientStatusComplete ||
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			nodeID := alloc.NodeID
			node, err := n.state.NodeByID(ws, nodeID)
			if err != nil || node == nil {
				n.logger.Error("looking up node failed", "node_id", nodeID, "error", err)
				return err

			}

			// Unblock any associated quota
			quota, err := n.allocQuota(alloc.ID)
			if err != nil {
				n.logger.Error("looking up quota associated with alloc failed", "alloc_id", alloc.ID, "error", err)
				return err
			}

			n.blockedEvals.UnblockClassAndQuota(node.ComputedClass, quota, index)
			n.blockedEvals.UnblockNode(node.ID, index)
		}
	}

	return nil
}

// applyAllocUpdateDesiredTransition is used to update the desired transitions
// of a set of allocations.
func (n *nomadFSM) applyAllocUpdateDesiredTransition(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "alloc_update_desired_transition"}, time.Now())
	var req structs.AllocUpdateDesiredTransitionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateAllocsDesiredTransitions(index, req.Allocs, req.Evals); err != nil {
		n.logger.Error("UpdateAllocsDesiredTransitions failed", "error", err)
		return err
	}

	n.handleUpsertedEvals(req.Evals)
	return nil
}

// applyReconcileSummaries reconciles summaries for all the jobs
func (n *nomadFSM) applyReconcileSummaries(buf []byte, index uint64) interface{} {
	if err := n.state.ReconcileJobSummaries(index); err != nil {
		return err
	}
	return n.reconcileQueuedAllocations(index)
}

// applyUpsertNodeEvent tracks the given node events.
func (n *nomadFSM) applyUpsertNodeEvent(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_node_events"}, time.Now())
	var req structs.EmitNodeEventsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode EmitNodeEventsRequest: %v", err))
	}

	if err := n.state.UpsertNodeEvents(index, req.NodeEvents); err != nil {
		n.logger.Error("failed to add node events", "error", err)
		return err
	}

	return nil
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
		n.logger.Error("UpsertVaultAccessor failed", "error", err)
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
		n.logger.Error("DeregisterVaultAccessor failed", "error", err)
		return err
	}

	return nil
}

func (n *nomadFSM) applyUpsertSIAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_si_accessor"}, time.Now())
	var request structs.SITokenAccessorsRequest
	if err := structs.Decode(buf, &request); err != nil {
		panic(errors.Wrap(err, "failed to decode request"))
	}

	if err := n.state.UpsertSITokenAccessors(index, request.Accessors); err != nil {
		n.logger.Error("UpsertSITokenAccessors failed", "error", err)
		return err
	}

	return nil
}

func (n *nomadFSM) applyDeregisterSIAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_si_accessor"}, time.Now())
	var request structs.SITokenAccessorsRequest
	if err := structs.Decode(buf, &request); err != nil {
		panic(errors.Wrap(err, "failed to decode request"))
	}

	if err := n.state.DeleteSITokenAccessors(index, request.Accessors); err != nil {
		n.logger.Error("DeregisterSITokenAccessor failed", "error", err)
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
		n.logger.Error("ApplyPlan failed", "error", err)
		return err
	}

	// Add evals for jobs that were preempted
	n.handleUpsertedEvals(req.PreemptionEvals)
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
		n.logger.Error("UpsertDeploymentStatusUpdate failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
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
		n.logger.Error("UpsertDeploymentPromotion failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
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
		n.logger.Error("UpsertDeploymentAllocHealth failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
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
		n.logger.Error("DeleteDeployment failed", "error", err)
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

	if err := n.state.UpdateJobStability(index, req.Namespace, req.JobID, req.JobVersion, req.Stable); err != nil {
		n.logger.Error("UpdateJobStability failed", "error", err)
		return err
	}

	return nil
}

// applyACLPolicyUpsert is used to upsert a set of policies
func (n *nomadFSM) applyACLPolicyUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_policy_upsert"}, time.Now())
	var req structs.ACLPolicyUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertACLPolicies(index, req.Policies); err != nil {
		n.logger.Error("UpsertACLPolicies failed", "error", err)
		return err
	}
	return nil
}

// applyACLPolicyDelete is used to delete a set of policies
func (n *nomadFSM) applyACLPolicyDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_policy_delete"}, time.Now())
	var req structs.ACLPolicyDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteACLPolicies(index, req.Names); err != nil {
		n.logger.Error("DeleteACLPolicies failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenUpsert is used to upsert a set of policies
func (n *nomadFSM) applyACLTokenUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_upsert"}, time.Now())
	var req structs.ACLTokenUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertACLTokens(index, req.Tokens); err != nil {
		n.logger.Error("UpsertACLTokens failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenDelete is used to delete a set of policies
func (n *nomadFSM) applyACLTokenDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_delete"}, time.Now())
	var req structs.ACLTokenDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteACLTokens(index, req.AccessorIDs); err != nil {
		n.logger.Error("DeleteACLTokens failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenBootstrap is used to bootstrap an ACL token
func (n *nomadFSM) applyACLTokenBootstrap(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_bootstrap"}, time.Now())
	var req structs.ACLTokenBootstrapRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.BootstrapACLTokens(index, req.ResetIndex, req.Token); err != nil {
		n.logger.Error("BootstrapACLToken failed", "error", err)
		return err
	}
	return nil
}

func (n *nomadFSM) applyAutopilotUpdate(buf []byte, index uint64) interface{} {
	var req structs.AutopilotSetConfigRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "autopilot"}, time.Now())

	if req.CAS {
		act, err := n.state.AutopilotCASConfig(index, req.Config.ModifyIndex, &req.Config)
		if err != nil {
			return err
		}
		return act
	}
	return n.state.AutopilotSetConfig(index, &req.Config)
}

func (n *nomadFSM) applySchedulerConfigUpdate(buf []byte, index uint64) interface{} {
	var req structs.SchedulerSetConfigRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_scheduler_config"}, time.Now())

	if req.CAS {
		applied, err := n.state.SchedulerCASConfig(index, req.Config.ModifyIndex, &req.Config)
		if err != nil {
			return err
		}
		return applied
	}
	return n.state.SchedulerSetConfig(index, &req.Config)
}

func (n *nomadFSM) applyCSIVolumeRegister(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_register"}, time.Now())

	if err := n.state.CSIVolumeRegister(index, req.Volumes); err != nil {
		n.logger.Error("CSIVolumeRegister failed", "error", err)
		return err
	}

	return nil
}

func (n *nomadFSM) applyCSIVolumeDeregister(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_deregister"}, time.Now())

	if err := n.state.CSIVolumeDeregister(index, req.RequestNamespace(), req.VolumeIDs); err != nil {
		n.logger.Error("CSIVolumeDeregister failed", "error", err)
		return err
	}

	return nil
}

func (n *nomadFSM) applyCSIVolumeClaim(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeClaimRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_claim"}, time.Now())

	ws := memdb.NewWatchSet()
	alloc, err := n.state.AllocByID(ws, req.AllocationID)
	if err != nil {
		n.logger.Error("AllocByID failed", "error", err)
		return err
	}
	if alloc == nil {
		n.logger.Error("AllocByID failed to find alloc", "alloc_id", req.AllocationID)
		if err != nil {
			return err
		}

		return structs.ErrUnknownAllocationPrefix
	}

	if err := n.state.CSIVolumeClaim(index, req.RequestNamespace(), req.VolumeID, alloc, req.Claim); err != nil {
		n.logger.Error("CSIVolumeClaim failed", "error", err)
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
	config := &state.StateStoreConfig{
		Logger: n.config.Logger,
		Region: n.config.Region,
	}
	newState, err := state.NewStateStore(config)
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
		snapType := SnapshotType(msgType[0])
		switch snapType {
		case TimeTableSnapshot:
			if err := n.timetable.Deserialize(dec); err != nil {
				return fmt.Errorf("time table deserialize failed: %v", err)
			}

		case NodeSnapshot:
			node := new(structs.Node)
			if err := dec.Decode(node); err != nil {
				return err
			}

			// Handle upgrade paths
			node.Canonicalize()

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
			 *   reflect.DeepEqual. Starting Nomad 0.4.1, job submission sanitizes
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

			// Handle upgrade path
			alloc.Canonicalize()

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

		case ServiceIdentityTokenAccessorSnapshot:
			accessor := new(structs.SITokenAccessor)
			if err := dec.Decode(accessor); err != nil {
				return err
			}
			if err := restore.SITokenAccessorRestore(accessor); err != nil {
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

		case ACLPolicySnapshot:
			policy := new(structs.ACLPolicy)
			if err := dec.Decode(policy); err != nil {
				return err
			}
			if err := restore.ACLPolicyRestore(policy); err != nil {
				return err
			}

		case ACLTokenSnapshot:
			token := new(structs.ACLToken)
			if err := dec.Decode(token); err != nil {
				return err
			}
			if err := restore.ACLTokenRestore(token); err != nil {
				return err
			}

		case SchedulerConfigSnapshot:
			schedConfig := new(structs.SchedulerConfiguration)
			if err := dec.Decode(schedConfig); err != nil {
				return err
			}
			if err := restore.SchedulerConfigRestore(schedConfig); err != nil {
				return err
			}

		case ClusterMetadataSnapshot:
			meta := new(structs.ClusterMetadata)
			if err := dec.Decode(meta); err != nil {
				return err
			}
			if err := restore.ClusterMetadataRestore(meta); err != nil {
				return err
			}

		case ScalingEventsSnapshot:
			jobScalingEvents := new(structs.JobScalingEvents)
			if err := dec.Decode(jobScalingEvents); err != nil {
				return err
			}

			if err := restore.ScalingEventsRestore(jobScalingEvents); err != nil {
				return err
			}

		case ScalingPolicySnapshot:
			scalingPolicy := new(structs.ScalingPolicy)
			if err := dec.Decode(scalingPolicy); err != nil {
				return err
			}

			if err := restore.ScalingPolicyRestore(scalingPolicy); err != nil {
				return err
			}

		case CSIPluginSnapshot:
			plugin := new(structs.CSIPlugin)
			if err := dec.Decode(plugin); err != nil {
				return err
			}

			if err := restore.CSIPluginRestore(plugin); err != nil {
				return err
			}

		case CSIVolumeSnapshot:
			plugin := new(structs.CSIVolume)
			if err := dec.Decode(plugin); err != nil {
				return err
			}

			if err := restore.CSIVolumeRestore(plugin); err != nil {
				return err
			}

		default:
			// Check if this is an enterprise only object being restored
			restorer, ok := n.enterpriseRestorers[snapType]
			if !ok {
				return fmt.Errorf("Unrecognized snapshot type: %v", msgType)
			}

			// Restore the enterprise only object
			if err := restorer(restore, dec); err != nil {
				return err
			}
		}
	}

	restore.Commit()

	// COMPAT Remove in 0.10
	// Clean up active deployments that do not have a job
	if err := n.failLeakedDeployments(newState); err != nil {
		return err
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

// failLeakedDeployments is used to fail deployments that do not have a job.
// This state is a broken invariant that should not occur since 0.8.X.
func (n *nomadFSM) failLeakedDeployments(state *state.StateStore) error {
	// Scan for deployments that are referencing a job that no longer exists.
	// This could happen if multiple deployments were created for a given job
	// and thus the older deployment leaks and then the job is removed.
	iter, err := state.Deployments(nil)
	if err != nil {
		return fmt.Errorf("failed to query deployments: %v", err)
	}

	dindex, err := state.Index("deployment")
	if err != nil {
		return fmt.Errorf("couldn't fetch index of deployments table: %v", err)
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		d := raw.(*structs.Deployment)

		// We are only looking for active deployments where the job no longer
		// exists
		if !d.Active() {
			continue
		}

		// Find the job
		job, err := state.JobByID(nil, d.Namespace, d.JobID)
		if err != nil {
			return fmt.Errorf("failed to lookup job %s from deployment %q: %v", d.JobID, d.ID, err)
		}

		// Job exists.
		if job != nil {
			continue
		}

		// Update the deployment to be terminal
		failed := d.Copy()
		failed.Status = structs.DeploymentStatusCancelled
		failed.StatusDescription = structs.DeploymentStatusDescriptionStoppedJob
		if err := state.UpsertDeployment(dindex, failed); err != nil {
			return fmt.Errorf("failed to mark leaked deployment %q as failed: %v", failed.ID, err)
		}
	}

	return nil
}

// reconcileQueuedAllocations re-calculates the queued allocations for every job that we
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

		// Nothing to do for queued allocations if the job is a parent periodic/parameterized job
		if job.IsParameterized() || job.IsPeriodic() {
			continue
		}
		planner := &scheduler.Harness{
			State: &snap.StateStore,
		}
		// Create an eval and mark it as requiring annotations and insert that as well
		eval := &structs.Evaluation{
			ID:             uuid.Generate(),
			Namespace:      job.Namespace,
			Priority:       job.Priority,
			Type:           job.Type,
			TriggeredBy:    structs.EvalTriggerJobRegister,
			JobID:          job.ID,
			JobModifyIndex: job.JobModifyIndex + 1,
			Status:         structs.EvalStatusPending,
			AnnotatePlan:   true,
		}
		snap.UpsertEvals(100, []*structs.Evaluation{eval})
		// Create the scheduler and run it
		sched, err := scheduler.NewScheduler(eval.Type, n.logger, snap, planner)
		if err != nil {
			return err
		}

		if err := sched.Process(eval); err != nil {
			return err
		}

		// Get the job summary from the fsm state store
		originalSummary, err := n.state.JobSummaryByID(ws, job.Namespace, job.ID)
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

func (n *nomadFSM) applyUpsertScalingEvent(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_scaling_event"}, time.Now())
	var req structs.ScalingEventRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertScalingEvent(index, &req); err != nil {
		n.logger.Error("UpsertScalingEvent failed", "error", err)
		return err
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
	if err := s.persistSITokenAccessors(sink, encoder); err != nil {
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
	if err := s.persistScalingPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistScalingEvents(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistCSIPlugins(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistCSIVolumes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistACLPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistACLTokens(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistEnterpriseTables(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistSchedulerConfig(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistClusterMetadata(sink, encoder); err != nil {
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

func (s *nomadSnapshot) persistSITokenAccessors(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	ws := memdb.NewWatchSet()
	accessors, err := s.snap.SITokenAccessors(ws)
	if err != nil {
		return err
	}

	for raw := accessors.Next(); raw != nil; raw = accessors.Next() {
		accessor := raw.(*structs.SITokenAccessor)
		sink.Write([]byte{byte(ServiceIdentityTokenAccessorSnapshot)})
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

func (s *nomadSnapshot) persistACLPolicies(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the policies
	ws := memdb.NewWatchSet()
	policies, err := s.snap.ACLPolicies(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := policies.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		policy := raw.(*structs.ACLPolicy)

		// Write out a policy registration
		sink.Write([]byte{byte(ACLPolicySnapshot)})
		if err := encoder.Encode(policy); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistACLTokens(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the policies
	ws := memdb.NewWatchSet()
	tokens, err := s.snap.ACLTokens(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := tokens.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		token := raw.(*structs.ACLToken)

		// Write out a token registration
		sink.Write([]byte{byte(ACLTokenSnapshot)})
		if err := encoder.Encode(token); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistSchedulerConfig(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get scheduler config
	_, schedConfig, err := s.snap.SchedulerConfig()
	if err != nil {
		return err
	}
	// Write out scheduler config
	sink.Write([]byte{byte(SchedulerConfigSnapshot)})
	if err := encoder.Encode(schedConfig); err != nil {
		return err
	}
	return nil
}

func (s *nomadSnapshot) persistClusterMetadata(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get the cluster metadata
	clusterMetadata, err := s.snap.ClusterMetadata()
	if err != nil {
		return err
	}

	// Write out the cluster metadata
	sink.Write([]byte{byte(ClusterMetadataSnapshot)})
	if err := encoder.Encode(clusterMetadata); err != nil {
		return err
	}

	return nil
}

func (s *nomadSnapshot) persistScalingPolicies(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the scaling policies
	ws := memdb.NewWatchSet()
	scalingPolicies, err := s.snap.ScalingPolicies(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := scalingPolicies.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		scalingPolicy := raw.(*structs.ScalingPolicy)

		// Write out a scaling policy snapshot
		sink.Write([]byte{byte(ScalingPolicySnapshot)})
		if err := encoder.Encode(scalingPolicy); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistScalingEvents(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	// Get all the scaling events
	ws := memdb.NewWatchSet()
	iter, err := s.snap.ScalingEvents(ws)
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
		events := raw.(*structs.JobScalingEvents)

		// Write out a scaling events snapshot
		sink.Write([]byte{byte(ScalingEventsSnapshot)})
		if err := encoder.Encode(events); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistCSIPlugins(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the CSI plugins
	ws := memdb.NewWatchSet()
	plugins, err := s.snap.CSIPlugins(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := plugins.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		plugin := raw.(*structs.CSIPlugin)

		// Write out a plugin snapshot
		sink.Write([]byte{byte(CSIPluginSnapshot)})
		if err := encoder.Encode(plugin); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistCSIVolumes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the CSI volumes
	ws := memdb.NewWatchSet()
	volumes, err := s.snap.CSIVolumes(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := volumes.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		volume := raw.(*structs.CSIVolume)

		// Write out a volume snapshot
		sink.Write([]byte{byte(CSIVolumeSnapshot)})
		if err := encoder.Encode(volume); err != nil {
			return err
		}
	}
	return nil
}

// Release is a no-op, as we just need to GC the pointer
// to the state store snapshot. There is nothing to explicitly
// cleanup.
func (s *nomadSnapshot) Release() {}
