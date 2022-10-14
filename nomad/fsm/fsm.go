package fsm

import (
	"fmt"
	"github.com/hashicorp/nomad/nomad/periodic"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/blockedevals"
	"github.com/hashicorp/nomad/nomad/evalbroker"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/timetable"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
)

const (
	// timeTableGranularity is the granularity of index to time tracking
	timeTableGranularity = 5 * time.Minute

	// timeTableLimit is the maximum limit of our tracking
	timeTableLimit = 72 * time.Hour
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

// FSM implements a finite state machine that is used along with Raft to
// provide strong consistency. We implement this outside the Server to
// avoid exposing this outside the package.
type FSM struct {
	evalBroker         *evalbroker.EvalBroker
	blockedEvals       *blockedevals.BlockedEvals
	periodicDispatcher *periodic.Dispatcher
	logger             hclog.Logger
	state              *state.StateStore
	timetable          *timetable.TimeTable

	// config is the FSM config
	config *Config

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

// Config is used to configure the FSM
type Config struct {
	// EvalBroker is the evaluation broker evaluations should be added to
	EvalBroker *evalbroker.EvalBroker

	// Periodic is the periodic job dispatcher that periodic jobs should be
	// added/removed from
	Periodic *periodic.Dispatcher

	// BlockedEvals is the blocked eval tracker that blocked evaluations should
	// be added to.
	Blocked *blockedevals.BlockedEvals

	// Logger is the logger used by the FSM
	Logger hclog.Logger

	// Region is the region of the server embedding the FSM
	Region string

	// EnableEventBroker specifies if the FSMs state store should enable
	// it's event publisher.
	EnableEventBroker bool

	// EventBufferSize is the amount of messages to hold in memory
	EventBufferSize int64
}

// NewFSM is used to construct a new FSM with a blank state.
func NewFSM(config *Config) (*FSM, error) {
	// Create a state store
	sconfig := &state.StateStoreConfig{
		Logger:          config.Logger,
		Region:          config.Region,
		EnablePublisher: config.EnableEventBroker,
		EventBufferSize: config.EventBufferSize,
	}
	stateStore, err := state.NewStateStore(sconfig)
	if err != nil {
		return nil, err
	}

	fsm := &FSM{
		evalBroker:          config.EvalBroker,
		periodicDispatcher:  config.Periodic,
		blockedEvals:        config.Blocked,
		logger:              config.Logger.Named("fsm"),
		config:              config,
		state:               stateStore,
		timetable:           timetable.NewTimeTable(timeTableGranularity, timeTableLimit),
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
func (n *FSM) Close() error {
	n.state.StopEventBroker()
	return nil
}

// State is used to return a handle to the current state
func (n *FSM) State() *state.StateStore {
	n.stateLock.RLock()
	defer n.stateLock.RUnlock()
	return n.state
}

// TimeTable returns the time table of transactions
func (n *FSM) TimeTable() *timetable.TimeTable {
	return n.timetable
}

func (n *FSM) Apply(log *raft.Log) interface{} {
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
		return n.applyUpsertNode(msgType, buf[1:], log.Index)
	case structs.NodeDeregisterRequestType:
		return n.applyDeregisterNode(msgType, buf[1:], log.Index)
	case structs.NodeUpdateStatusRequestType:
		return n.applyStatusUpdate(msgType, buf[1:], log.Index)
	case structs.NodeUpdateDrainRequestType:
		return n.applyDrainUpdate(msgType, buf[1:], log.Index)
	case structs.JobRegisterRequestType:
		return n.applyUpsertJob(msgType, buf[1:], log.Index)
	case structs.JobDeregisterRequestType:
		return n.applyDeregisterJob(msgType, buf[1:], log.Index)
	case structs.EvalUpdateRequestType:
		return n.applyUpdateEval(msgType, buf[1:], log.Index)
	case structs.EvalDeleteRequestType:
		return n.applyDeleteEval(buf[1:], log.Index)
	case structs.AllocUpdateRequestType:
		return n.applyAllocUpdate(msgType, buf[1:], log.Index)
	case structs.AllocClientUpdateRequestType:
		return n.applyAllocClientUpdate(msgType, buf[1:], log.Index)
	case structs.ReconcileJobSummariesRequestType:
		return n.applyReconcileSummaries(buf[1:], log.Index)
	case structs.VaultAccessorRegisterRequestType:
		return n.applyUpsertVaultAccessor(buf[1:], log.Index)
	case structs.VaultAccessorDeregisterRequestType:
		return n.applyDeregisterVaultAccessor(buf[1:], log.Index)
	case structs.ApplyPlanResultsRequestType:
		return n.applyPlanResults(msgType, buf[1:], log.Index)
	case structs.DeploymentStatusUpdateRequestType:
		return n.applyDeploymentStatusUpdate(msgType, buf[1:], log.Index)
	case structs.DeploymentPromoteRequestType:
		return n.applyDeploymentPromotion(msgType, buf[1:], log.Index)
	case structs.DeploymentAllocHealthRequestType:
		return n.applyDeploymentAllocHealth(msgType, buf[1:], log.Index)
	case structs.DeploymentDeleteRequestType:
		return n.applyDeploymentDelete(buf[1:], log.Index)
	case structs.JobStabilityRequestType:
		return n.applyJobStability(buf[1:], log.Index)
	case structs.ACLPolicyUpsertRequestType:
		return n.applyACLPolicyUpsert(msgType, buf[1:], log.Index)
	case structs.ACLPolicyDeleteRequestType:
		return n.applyACLPolicyDelete(msgType, buf[1:], log.Index)
	case structs.ACLTokenUpsertRequestType:
		return n.applyACLTokenUpsert(msgType, buf[1:], log.Index)
	case structs.ACLTokenDeleteRequestType:
		return n.applyACLTokenDelete(msgType, buf[1:], log.Index)
	case structs.ACLTokenBootstrapRequestType:
		return n.applyACLTokenBootstrap(msgType, buf[1:], log.Index)
	case structs.AutopilotRequestType:
		return n.applyAutopilotUpdate(buf[1:], log.Index)
	case structs.UpsertNodeEventsType:
		return n.applyUpsertNodeEvent(msgType, buf[1:], log.Index)
	case structs.JobBatchDeregisterRequestType:
		return n.applyBatchDeregisterJob(msgType, buf[1:], log.Index)
	case structs.AllocUpdateDesiredTransitionRequestType:
		return n.applyAllocUpdateDesiredTransition(msgType, buf[1:], log.Index)
	case structs.NodeUpdateEligibilityRequestType:
		return n.applyNodeEligibilityUpdate(msgType, buf[1:], log.Index)
	case structs.BatchNodeUpdateDrainRequestType:
		return n.applyBatchDrainUpdate(msgType, buf[1:], log.Index)
	case structs.SchedulerConfigRequestType:
		return n.applySchedulerConfigUpdate(buf[1:], log.Index)
	case structs.NodeBatchDeregisterRequestType:
		return n.applyDeregisterNodeBatch(msgType, buf[1:], log.Index)
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
	case structs.CSIVolumeClaimBatchRequestType:
		return n.applyCSIVolumeBatchClaim(buf[1:], log.Index)
	case structs.CSIPluginDeleteRequestType:
		return n.applyCSIPluginDelete(buf[1:], log.Index)
	case structs.NamespaceUpsertRequestType:
		return n.applyNamespaceUpsert(buf[1:], log.Index)
	case structs.NamespaceDeleteRequestType:
		return n.applyNamespaceDelete(buf[1:], log.Index)
	// COMPAT(1.0): These messages were added and removed during the 1.0-beta
	// series and should not be immediately reused for other purposes
	case structs.EventSinkUpsertRequestType,
		structs.EventSinkDeleteRequestType,
		structs.BatchEventSinkUpdateProgressType:
		return nil
	case structs.OneTimeTokenUpsertRequestType:
		return n.applyOneTimeTokenUpsert(msgType, buf[1:], log.Index)
	case structs.OneTimeTokenDeleteRequestType:
		return n.applyOneTimeTokenDelete(msgType, buf[1:], log.Index)
	case structs.OneTimeTokenExpireRequestType:
		return n.applyOneTimeTokenExpire(msgType, buf[1:], log.Index)
	case structs.ServiceRegistrationUpsertRequestType:
		return n.applyUpsertServiceRegistrations(msgType, buf[1:], log.Index)
	case structs.ServiceRegistrationDeleteByIDRequestType:
		return n.applyDeleteServiceRegistrationByID(msgType, buf[1:], log.Index)
	case structs.ServiceRegistrationDeleteByNodeIDRequestType:
		return n.applyDeleteServiceRegistrationByNodeID(msgType, buf[1:], log.Index)
	case structs.VarApplyStateRequestType:
		return n.applyVariableOperation(msgType, buf[1:], log.Index)
	case structs.RootKeyMetaUpsertRequestType:
		return n.applyRootKeyMetaUpsert(msgType, buf[1:], log.Index)
	case structs.RootKeyMetaDeleteRequestType:
		return n.applyRootKeyMetaDelete(msgType, buf[1:], log.Index)
	case structs.ACLRolesUpsertRequestType:
		return n.applyACLRolesUpsert(msgType, buf[1:], log.Index)
	case structs.ACLRolesDeleteByIDRequestType:
		return n.applyACLRolesDeleteByID(msgType, buf[1:], log.Index)
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

func (n *FSM) applyClusterMetadata(buf []byte, index uint64) interface{} {
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

func (n *FSM) applyUpsertNode(reqType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "register_node"}, time.Now())
	var req structs.NodeRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Handle upgrade paths
	req.Node.Canonicalize()

	if err := n.state.UpsertNode(reqType, index, req.Node); err != nil {
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

func (n *FSM) applyDeregisterNode(reqType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_node"}, time.Now())
	var req structs.NodeDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNode(reqType, index, []string{req.NodeID}); err != nil {
		n.logger.Error("DeleteNode failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyDeregisterNodeBatch(reqType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_deregister_node"}, time.Now())
	var req structs.NodeBatchDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNode(reqType, index, req.NodeIDs); err != nil {
		n.logger.Error("DeleteNode failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyStatusUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_status_update"}, time.Now())
	var req structs.NodeUpdateStatusRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateNodeStatus(msgType, index, req.NodeID, req.Status, req.UpdatedAt, req.NodeEvent); err != nil {
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

func (n *FSM) applyDrainUpdate(reqType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "node_drain_update"}, time.Now())
	var req structs.NodeUpdateDrainRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	accessorId := ""
	if req.AuthToken != "" {
		token, err := n.state.ACLTokenBySecretID(nil, req.AuthToken)
		if err != nil {
			n.logger.Error("error looking up ACL token from drain update", "error", err)
			return fmt.Errorf("error looking up ACL token: %v", err)
		}
		if token == nil {
			n.logger.Error("token did not exist during node drain update")
			return fmt.Errorf("token did not exist during node drain update")
		}
		accessorId = token.AccessorID
	}

	if err := n.state.UpdateNodeDrain(reqType, index, req.NodeID, req.DrainStrategy, req.MarkEligible, req.UpdatedAt,
		req.NodeEvent, req.Meta, accessorId); err != nil {
		n.logger.Error("UpdateNodeDrain failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyBatchDrainUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_node_drain_update"}, time.Now())
	var req structs.BatchNodeUpdateDrainRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.BatchUpdateNodeDrain(msgType, index, req.UpdatedAt, req.Updates, req.NodeEvents); err != nil {
		n.logger.Error("BatchUpdateNodeDrain failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyNodeEligibilityUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
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

	if err := n.state.UpdateNodeEligibility(msgType, index, req.NodeID, req.Eligibility, req.UpdatedAt, req.NodeEvent); err != nil {
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

func (n *FSM) applyUpsertJob(msgType structs.MessageType, buf []byte, index uint64) interface{} {
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

	if err := n.state.UpsertJob(msgType, index, req.Job); err != nil {
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

	// COMPAT: Prior to Nomad 0.12.x evaluations were submitted in a separate Raft log,
	// so this may be nil during server upgrades.
	if req.Eval != nil {
		req.Eval.JobModifyIndex = index
		if err := n.upsertEvals(msgType, index, []*structs.Evaluation{req.Eval}); err != nil {
			return err
		}
	}

	return nil
}

func (n *FSM) applyDeregisterJob(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_job"}, time.Now())
	var req structs.JobDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	err := n.state.WithWriteTransaction(msgType, index, func(tx state.Txn) error {
		err := n.handleJobDeregister(index, req.JobID, req.Namespace, req.Purge, req.NoShutdownDelay, tx)

		if err != nil {
			n.logger.Error("deregistering job failed",
				"error", err, "job", req.JobID, "namespace", req.Namespace)
			return err
		}

		return nil
	})

	// COMPAT: Prior to Nomad 0.12.x evaluations were submitted in a separate Raft log,
	// so this may be nil during server upgrades.
	// always attempt upsert eval even if job deregister fail
	if req.Eval != nil {
		req.Eval.JobModifyIndex = index
		if err := n.upsertEvals(msgType, index, []*structs.Evaluation{req.Eval}); err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (n *FSM) applyBatchDeregisterJob(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "batch_deregister_job"}, time.Now())
	var req structs.JobBatchDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Perform all store updates atomically to ensure a consistent view for store readers.
	// A partial update may increment the snapshot index, allowing eval brokers to process
	// evals for jobs whose deregistering didn't get committed yet.
	err := n.state.WithWriteTransaction(msgType, index, func(tx state.Txn) error {
		for jobNS, options := range req.Jobs {
			if err := n.handleJobDeregister(index, jobNS.ID, jobNS.Namespace, options.Purge, false, tx); err != nil {
				n.logger.Error("deregistering job failed", "job", jobNS.ID, "error", err)
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

// handleJobDeregister is used to deregister a job. Leaves error logging up to
// caller.
func (n *FSM) handleJobDeregister(index uint64, jobID, namespace string, purge bool, noShutdownDelay bool, tx state.Txn) error {
	// If it is periodic remove it from the dispatcher
	if err := n.periodicDispatcher.Remove(namespace, jobID); err != nil {
		return fmt.Errorf("periodicDispatcher.Remove failed: %w", err)
	}

	if noShutdownDelay {
		ws := memdb.NewWatchSet()
		allocs, err := n.state.AllocsByJob(ws, namespace, jobID, false)
		if err != nil {
			return err
		}
		transition := &structs.DesiredTransition{NoShutdownDelay: pointer.Of(true)}
		for _, alloc := range allocs {
			err := n.state.UpdateAllocDesiredTransitionTxn(tx, index, alloc.ID, transition)
			if err != nil {
				return err
			}
			err = tx.Insert("index", &state.IndexEntry{Key: "allocs", Value: index})
			if err != nil {
				return fmt.Errorf("index update failed: %v", err)
			}
		}
	}

	if purge {
		if err := n.state.DeleteJobTxn(index, namespace, jobID, tx); err != nil {
			return fmt.Errorf("DeleteJob failed: %w", err)
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
			return fmt.Errorf("JobByID lookup failed: %w", err)
		}

		if current == nil {
			return fmt.Errorf("job %q in namespace %q doesn't exist to be deregistered", jobID, namespace)
		}

		stopped := current.Copy()
		stopped.Stop = true

		if err := n.state.UpsertJobTxn(index, stopped, tx); err != nil {
			return fmt.Errorf("UpsertJob failed: %w", err)
		}
	}

	return nil
}

func (n *FSM) applyUpdateEval(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "update_eval"}, time.Now())
	var req structs.EvalUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	return n.upsertEvals(msgType, index, req.Evals)
}

func (n *FSM) upsertEvals(msgType structs.MessageType, index uint64, evals []*structs.Evaluation) error {
	if err := n.state.UpsertEvals(msgType, index, evals); err != nil {
		n.logger.Error("UpsertEvals failed", "error", err)
		return err
	}

	n.handleUpsertedEvals(evals)
	return nil
}

// handleUpsertingEval is a helper for taking action after upserting
// evaluations.
func (n *FSM) handleUpsertedEvals(evals []*structs.Evaluation) {
	for _, eval := range evals {
		n.handleUpsertedEval(eval)
	}
}

// handleUpsertingEval is a helper for taking action after upserting an eval.
func (n *FSM) handleUpsertedEval(eval *structs.Evaluation) {
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

func (n *FSM) applyDeleteEval(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "delete_eval"}, time.Now())
	var req structs.EvalReapRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteEval(index, req.Evals, req.Allocs, req.UserInitiated); err != nil {
		n.logger.Error("DeleteEval failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyAllocUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
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

	if err := n.state.UpsertAllocs(msgType, index, req.Alloc); err != nil {
		n.logger.Error("UpsertAllocs failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyAllocClientUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
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
	if err := n.state.UpdateAllocsFromClient(msgType, index, req.Alloc); err != nil {
		n.logger.Error("UpdateAllocFromClient failed", "error", err)
		return err
	}

	// Update any evals
	if len(req.Evals) > 0 {
		if err := n.upsertEvals(msgType, index, req.Evals); err != nil {
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
func (n *FSM) applyAllocUpdateDesiredTransition(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "alloc_update_desired_transition"}, time.Now())
	var req structs.AllocUpdateDesiredTransitionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateAllocsDesiredTransitions(msgType, index, req.Allocs, req.Evals); err != nil {
		n.logger.Error("UpdateAllocsDesiredTransitions failed", "error", err)
		return err
	}

	n.handleUpsertedEvals(req.Evals)
	return nil
}

// applyReconcileSummaries reconciles summaries for all the jobs
func (n *FSM) applyReconcileSummaries(buf []byte, index uint64) interface{} {
	if err := n.state.ReconcileJobSummaries(index); err != nil {
		return err
	}
	return n.reconcileQueuedAllocations(index)
}

// applyUpsertNodeEvent tracks the given node events.
func (n *FSM) applyUpsertNodeEvent(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_node_events"}, time.Now())
	var req structs.EmitNodeEventsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode EmitNodeEventsRequest: %v", err))
	}

	if err := n.state.UpsertNodeEvents(msgType, index, req.NodeEvents); err != nil {
		n.logger.Error("failed to add node events", "error", err)
		return err
	}

	return nil
}

// applyUpsertVaultAccessor stores the Vault accessors for a given allocation
// and task
func (n *FSM) applyUpsertVaultAccessor(buf []byte, index uint64) interface{} {
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
func (n *FSM) applyDeregisterVaultAccessor(buf []byte, index uint64) interface{} {
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

func (n *FSM) applyUpsertSIAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "upsert_si_accessor"}, time.Now())
	var request structs.SITokenAccessorsRequest
	if err := structs.Decode(buf, &request); err != nil {
		panic(fmt.Errorf("failed to decode request: %w", err))
	}

	if err := n.state.UpsertSITokenAccessors(index, request.Accessors); err != nil {
		n.logger.Error("UpsertSITokenAccessors failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyDeregisterSIAccessor(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "deregister_si_accessor"}, time.Now())
	var request structs.SITokenAccessorsRequest
	if err := structs.Decode(buf, &request); err != nil {
		panic(fmt.Errorf("failed to decode request: %w", err))
	}

	if err := n.state.DeleteSITokenAccessors(index, request.Accessors); err != nil {
		n.logger.Error("DeregisterSITokenAccessor failed", "error", err)
		return err
	}

	return nil
}

// applyPlanApply applies the results of a plan application
func (n *FSM) applyPlanResults(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_plan_results"}, time.Now())
	var req structs.ApplyPlanResultsRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertPlanResults(msgType, index, &req); err != nil {
		n.logger.Error("ApplyPlan failed", "error", err)
		return err
	}

	// Add evals for jobs that were preempted
	n.handleUpsertedEvals(req.PreemptionEvals)
	return nil
}

// applyDeploymentStatusUpdate is used to update the status of an existing
// deployment
func (n *FSM) applyDeploymentStatusUpdate(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_status_update"}, time.Now())
	var req structs.DeploymentStatusUpdateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentStatus(msgType, index, &req); err != nil {
		n.logger.Error("UpsertDeploymentStatusUpdate failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
	return nil
}

// applyDeploymentPromotion is used to promote canaries in a deployment
func (n *FSM) applyDeploymentPromotion(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_promotion"}, time.Now())
	var req structs.ApplyDeploymentPromoteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentPromotion(msgType, index, &req); err != nil {
		n.logger.Error("UpsertDeploymentPromotion failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
	return nil
}

// applyDeploymentAllocHealth is used to set the health of allocations as part
// of a deployment
func (n *FSM) applyDeploymentAllocHealth(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_deployment_alloc_health"}, time.Now())
	var req structs.ApplyDeploymentAllocHealthRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpdateDeploymentAllocHealth(msgType, index, &req); err != nil {
		n.logger.Error("UpsertDeploymentAllocHealth failed", "error", err)
		return err
	}

	n.handleUpsertedEval(req.Eval)
	return nil
}

// applyDeploymentDelete is used to delete a set of deployments
func (n *FSM) applyDeploymentDelete(buf []byte, index uint64) interface{} {
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
func (n *FSM) applyJobStability(buf []byte, index uint64) interface{} {
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
func (n *FSM) applyACLPolicyUpsert(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_policy_upsert"}, time.Now())
	var req structs.ACLPolicyUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertACLPolicies(msgType, index, req.Policies); err != nil {
		n.logger.Error("UpsertACLPolicies failed", "error", err)
		return err
	}
	return nil
}

// applyACLPolicyDelete is used to delete a set of policies
func (n *FSM) applyACLPolicyDelete(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_policy_delete"}, time.Now())
	var req structs.ACLPolicyDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteACLPolicies(msgType, index, req.Names); err != nil {
		n.logger.Error("DeleteACLPolicies failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenUpsert is used to upsert a set of policies
func (n *FSM) applyACLTokenUpsert(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_upsert"}, time.Now())
	var req structs.ACLTokenUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertACLTokens(msgType, index, req.Tokens); err != nil {
		n.logger.Error("UpsertACLTokens failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenDelete is used to delete a set of policies
func (n *FSM) applyACLTokenDelete(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_delete"}, time.Now())
	var req structs.ACLTokenDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteACLTokens(msgType, index, req.AccessorIDs); err != nil {
		n.logger.Error("DeleteACLTokens failed", "error", err)
		return err
	}
	return nil
}

// applyACLTokenBootstrap is used to bootstrap an ACL token
func (n *FSM) applyACLTokenBootstrap(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_token_bootstrap"}, time.Now())
	var req structs.ACLTokenBootstrapRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.BootstrapACLTokens(msgType, index, req.ResetIndex, req.Token); err != nil {
		n.logger.Error("BootstrapACLToken failed", "error", err)
		return err
	}
	return nil
}

// applyOneTimeTokenUpsert is used to upsert a one-time token
func (n *FSM) applyOneTimeTokenUpsert(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_one_time_token_upsert"}, time.Now())
	var req structs.OneTimeToken
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertOneTimeToken(msgType, index, &req); err != nil {
		n.logger.Error("UpsertOneTimeToken failed", "error", err)
		return err
	}
	return nil
}

// applyOneTimeTokenDelete is used to delete a set of one-time tokens
func (n *FSM) applyOneTimeTokenDelete(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_one_time_token_delete"}, time.Now())
	var req structs.OneTimeTokenDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteOneTimeTokens(msgType, index, req.AccessorIDs); err != nil {
		n.logger.Error("DeleteOneTimeTokens failed", "error", err)
		return err
	}
	return nil
}

// applyOneTimeTokenExpire is used to delete a set of one-time tokens
func (n *FSM) applyOneTimeTokenExpire(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_one_time_token_expire"}, time.Now())
	var req structs.OneTimeTokenExpireRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.ExpireOneTimeTokens(msgType, index, req.Timestamp); err != nil {
		n.logger.Error("ExpireOneTimeTokens failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyAutopilotUpdate(buf []byte, index uint64) interface{} {
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

func (n *FSM) applySchedulerConfigUpdate(buf []byte, index uint64) interface{} {
	var req structs.SchedulerSetConfigRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_scheduler_config"}, time.Now())

	req.Config.Canonicalize()

	if req.CAS {
		applied, err := n.state.SchedulerCASConfig(index, req.Config.ModifyIndex, &req.Config)
		if err != nil {
			return err
		}
		return applied
	}
	return n.state.SchedulerSetConfig(index, &req.Config)
}

func (n *FSM) applyCSIVolumeRegister(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeRegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_register"}, time.Now())

	if err := n.state.UpsertCSIVolume(index, req.Volumes); err != nil {
		n.logger.Error("CSIVolumeRegister failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyCSIVolumeDeregister(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeDeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_deregister"}, time.Now())

	if err := n.state.CSIVolumeDeregister(index, req.RequestNamespace(), req.VolumeIDs, req.Force); err != nil {
		n.logger.Error("CSIVolumeDeregister failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyCSIVolumeBatchClaim(buf []byte, index uint64) interface{} {
	var batch *structs.CSIVolumeClaimBatchRequest
	if err := structs.Decode(buf, &batch); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_batch_claim"}, time.Now())

	for _, req := range batch.Claims {
		err := n.state.CSIVolumeClaim(index, req.RequestNamespace(),
			req.VolumeID, req.ToClaim())
		if err != nil {
			n.logger.Error("CSIVolumeClaim for batch failed", "error", err)
			return err // note: fails the remaining batch
		}
	}
	return nil
}

func (n *FSM) applyCSIVolumeClaim(buf []byte, index uint64) interface{} {
	var req structs.CSIVolumeClaimRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_volume_claim"}, time.Now())

	if err := n.state.CSIVolumeClaim(index, req.RequestNamespace(), req.VolumeID, req.ToClaim()); err != nil {
		n.logger.Error("CSIVolumeClaim failed", "error", err)
		return err
	}
	return nil
}

func (n *FSM) applyCSIPluginDelete(buf []byte, index uint64) interface{} {
	var req structs.CSIPluginDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_csi_plugin_delete"}, time.Now())

	if err := n.state.DeleteCSIPlugin(index, req.ID); err != nil {
		// "plugin in use" is an error for the state store but not for typical
		// callers, so reduce log noise by not logging that case here
		if err.Error() != "plugin in use" {
			n.logger.Error("DeleteCSIPlugin failed", "error", err)
		}
		return err
	}
	return nil
}

// applyNamespaceUpsert is used to upsert a set of namespaces
func (n *FSM) applyNamespaceUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_namespace_upsert"}, time.Now())
	var req structs.NamespaceUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	var trigger []string
	for _, ns := range req.Namespaces {
		old, err := n.state.NamespaceByName(nil, ns.Name)
		if err != nil {
			n.logger.Error("namespace lookup failed", "error", err)
			return err
		}

		// If we are changing the quota on a namespace trigger evals for the
		// older quota.
		if old != nil && old.Quota != "" && old.Quota != ns.Quota {
			trigger = append(trigger, old.Quota)
		}
	}

	if err := n.state.UpsertNamespaces(index, req.Namespaces); err != nil {
		n.logger.Error("UpsertNamespaces failed", "error", err)
		return err
	}

	// Send the unblocks
	for _, quota := range trigger {
		n.blockedEvals.UnblockQuota(quota, index)
	}

	return nil
}

// applyNamespaceDelete is used to delete a set of namespaces
func (n *FSM) applyNamespaceDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_namespace_delete"}, time.Now())
	var req structs.NamespaceDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNamespaces(index, req.Namespaces); err != nil {
		n.logger.Error("DeleteNamespaces failed", "error", err)
	}

	return nil
}

func (n *FSM) Snapshot() (raft.FSMSnapshot, error) {
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

// Restore implements the raft.FSM interface, which doesn't support a
// filtering parameter
func (n *FSM) Restore(old io.ReadCloser) error {
	return n.restoreImpl(old, nil)
}

// RestoreWithFilter includes a set of bexpr filter evaluators, so
// that we can create a FSM that excludes a portion of a snapshot
// (typically for debugging and testing)
func (n *FSM) RestoreWithFilter(old io.ReadCloser, filter *Filter) error {
	return n.restoreImpl(old, filter)
}

func (n *FSM) restoreImpl(old io.ReadCloser, filter *Filter) error {
	defer old.Close()

	// Create a new state store
	config := &state.StateStoreConfig{
		Logger:          n.config.Logger,
		Region:          n.config.Region,
		EnablePublisher: n.config.EnableEventBroker,
		EventBufferSize: n.config.EventBufferSize,
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
			if filter.Include(node) {
				node.Canonicalize() // Handle upgrade paths
				if err := restore.NodeRestore(node); err != nil {
					return err
				}
			}

		case JobSnapshot:
			job := new(structs.Job)
			if err := dec.Decode(job); err != nil {
				return err
			}
			if filter.Include(job) {
				/* Handle upgrade paths:
				 * - Empty maps and slices should be treated as nil to avoid
				 *   un-intended destructive updates in scheduler since we use
				 *   reflect.DeepEqual. Job submission sanitizes the incoming job.
				 * - Migrate from old style upgrade stanza that used only a stagger.
				 */
				job.Canonicalize()
				if err := restore.JobRestore(job); err != nil {
					return err
				}
			}

		case EvalSnapshot:
			eval := new(structs.Evaluation)
			if err := dec.Decode(eval); err != nil {
				return err
			}
			if filter.Include(eval) {
				if err := restore.EvalRestore(eval); err != nil {
					return err
				}
			}

		case AllocSnapshot:
			alloc := new(structs.Allocation)
			if err := dec.Decode(alloc); err != nil {
				return err
			}
			if filter.Include(alloc) {
				alloc.Canonicalize() // Handle upgrade path
				if err := restore.AllocRestore(alloc); err != nil {
					return err
				}
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
			if filter.Include(launch) {
				if err := restore.PeriodicLaunchRestore(launch); err != nil {
					return err
				}
			}

		case JobSummarySnapshot:
			summary := new(structs.JobSummary)
			if err := dec.Decode(summary); err != nil {
				return err
			}
			if filter.Include(summary) {
				if err := restore.JobSummaryRestore(summary); err != nil {
					return err
				}
			}

		case VaultAccessorSnapshot:
			accessor := new(structs.VaultAccessor)
			if err := dec.Decode(accessor); err != nil {
				return err
			}
			if filter.Include(accessor) {
				if err := restore.VaultAccessorRestore(accessor); err != nil {
					return err
				}
			}

		case ServiceIdentityTokenAccessorSnapshot:
			accessor := new(structs.SITokenAccessor)
			if err := dec.Decode(accessor); err != nil {
				return err
			}
			if filter.Include(accessor) {
				if err := restore.SITokenAccessorRestore(accessor); err != nil {
					return err
				}
			}

		case JobVersionSnapshot:
			version := new(structs.Job)
			if err := dec.Decode(version); err != nil {
				return err
			}
			if filter.Include(version) {
				if err := restore.JobVersionRestore(version); err != nil {
					return err
				}
			}

		case DeploymentSnapshot:
			deployment := new(structs.Deployment)
			if err := dec.Decode(deployment); err != nil {
				return err
			}
			if filter.Include(deployment) {
				if err := restore.DeploymentRestore(deployment); err != nil {
					return err
				}
			}

		case ACLPolicySnapshot:
			policy := new(structs.ACLPolicy)
			if err := dec.Decode(policy); err != nil {
				return err
			}
			if filter.Include(policy) {
				if err := restore.ACLPolicyRestore(policy); err != nil {
					return err
				}
			}

		case ACLTokenSnapshot:
			token := new(structs.ACLToken)
			if err := dec.Decode(token); err != nil {
				return err
			}
			if filter.Include(token) {
				if err := restore.ACLTokenRestore(token); err != nil {
					return err
				}
			}

		case SchedulerConfigSnapshot:
			schedConfig := new(structs.SchedulerConfiguration)
			if err := dec.Decode(schedConfig); err != nil {
				return err
			}
			schedConfig.Canonicalize()
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
			if filter.Include(jobScalingEvents) {
				if err := restore.ScalingEventsRestore(jobScalingEvents); err != nil {
					return err
				}
			}

		case ScalingPolicySnapshot:
			scalingPolicy := new(structs.ScalingPolicy)
			if err := dec.Decode(scalingPolicy); err != nil {
				return err
			}
			if filter.Include(scalingPolicy) {
				// Handle upgrade path:
				//   - Set policy type if empty
				scalingPolicy.Canonicalize()
				if err := restore.ScalingPolicyRestore(scalingPolicy); err != nil {
					return err
				}
			}

		case CSIPluginSnapshot:
			plugin := new(structs.CSIPlugin)
			if err := dec.Decode(plugin); err != nil {
				return err
			}
			if filter.Include(plugin) {
				if err := restore.CSIPluginRestore(plugin); err != nil {
					return err
				}
			}

		case CSIVolumeSnapshot:
			volume := new(structs.CSIVolume)
			if err := dec.Decode(volume); err != nil {
				return err
			}
			if filter.Include(volume) {
				if err := restore.CSIVolumeRestore(volume); err != nil {
					return err
				}
			}

		case NamespaceSnapshot:
			namespace := new(structs.Namespace)
			if err := dec.Decode(namespace); err != nil {
				return err
			}
			if err := restore.NamespaceRestore(namespace); err != nil {
				return err
			}

		// COMPAT(1.0): Allow 1.0-beta clusterers to gracefully handle
		case EventSinkSnapshot:
			return nil

		case ServiceRegistrationSnapshot:
			serviceRegistration := new(structs.ServiceRegistration)
			if err := dec.Decode(serviceRegistration); err != nil {
				return err
			}
			if filter.Include(serviceRegistration) {
				// Perform the restoration.
				if err := restore.ServiceRegistrationRestore(serviceRegistration); err != nil {
					return err
				}
			}

		case VariablesSnapshot:
			variable := new(structs.VariableEncrypted)
			if err := dec.Decode(variable); err != nil {
				return err
			}

			if err := restore.VariablesRestore(variable); err != nil {
				return err
			}

		case VariablesQuotaSnapshot:
			quota := new(structs.VariablesQuota)
			if err := dec.Decode(quota); err != nil {
				return err
			}

			if err := restore.VariablesQuotaRestore(quota); err != nil {
				return err
			}

		case RootKeyMetaSnapshot:
			keyMeta := new(structs.RootKeyMeta)
			if err := dec.Decode(keyMeta); err != nil {
				return err
			}

			if err := restore.RootKeyMetaRestore(keyMeta); err != nil {
				return err
			}
		case ACLRoleSnapshot:

			// Create a new ACLRole object, so we can decode the message into
			// it.
			aclRole := new(structs.ACLRole)

			if err := dec.Decode(aclRole); err != nil {
				return err
			}

			// Perform the restoration.
			if err := restore.ACLRoleRestore(aclRole); err != nil {
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

	if err := restore.Commit(); err != nil {
		return err
	}

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
func (n *FSM) failLeakedDeployments(store *state.StateStore) error {
	// Scan for deployments that are referencing a job that no longer exists.
	// This could happen if multiple deployments were created for a given job
	// and thus the older deployment leaks and then the job is removed.
	iter, err := store.Deployments(nil, state.SortDefault)
	if err != nil {
		return fmt.Errorf("failed to query deployments: %v", err)
	}

	dindex, err := store.Index("deployment")
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
		job, err := store.JobByID(nil, d.Namespace, d.JobID)
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
		if err := store.UpsertDeployment(dindex, failed); err != nil {
			return fmt.Errorf("failed to mark leaked deployment %q as failed: %v", failed.ID, err)
		}
	}

	return nil
}

// reconcileQueuedAllocations re-calculates the queued allocations for every job that we
// created a Job Summary during the snap shot restore
func (n *FSM) reconcileQueuedAllocations(index uint64) error {
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
		// Ignore eval event creation during snapshot restore
		snap.UpsertEvals(structs.IgnoreUnknownTypeFlag, 100, []*structs.Evaluation{eval})
		// Create the scheduler and run it
		sched, err := scheduler.NewScheduler(eval.Type, n.logger, nil, snap, planner)
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

func (n *FSM) applyUpsertScalingEvent(buf []byte, index uint64) interface{} {
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

func (n *FSM) applyUpsertServiceRegistrations(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_service_registration_upsert"}, time.Now())
	var req structs.ServiceRegistrationUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertServiceRegistrations(msgType, index, req.Services); err != nil {
		n.logger.Error("UpsertServiceRegistrations failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyDeleteServiceRegistrationByID(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_service_registration_delete_id"}, time.Now())
	var req structs.ServiceRegistrationDeleteByIDRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteServiceRegistrationByID(msgType, index, req.RequestNamespace(), req.ID); err != nil {
		n.logger.Error("DeleteServiceRegistrationByID failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyDeleteServiceRegistrationByNodeID(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_service_registration_delete_node_id"}, time.Now())
	var req structs.ServiceRegistrationDeleteByNodeIDRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteServiceRegistrationByNodeID(msgType, index, req.NodeID); err != nil {
		n.logger.Error("DeleteServiceRegistrationByNodeID failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyACLRolesUpsert(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_role_upsert"}, time.Now())
	var req structs.ACLRolesUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertACLRoles(msgType, index, req.ACLRoles, req.AllowMissingPolicies); err != nil {
		n.logger.Error("UpsertACLRoles failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyACLRolesDeleteByID(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_acl_role_delete_by_id"}, time.Now())
	var req structs.ACLRolesDeleteByIDRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteACLRolesByID(msgType, index, req.ACLRoleIDs); err != nil {
		n.logger.Error("DeleteACLRolesByID failed", "error", err)
		return err
	}

	return nil
}

type Filter struct {
	evaluator *bexpr.Evaluator
}

func NewFilter(expr string) (*Filter, error) {
	if expr == "" {
		return nil, nil
	}
	evaluator, err := bexpr.CreateEvaluator(expr)
	if err != nil {
		return nil, err
	}
	return &Filter{evaluator: evaluator}, nil
}

func (f *Filter) Include(item interface{}) bool {
	if f == nil {
		return true
	}
	ok, err := f.evaluator.Evaluate(item)
	if !ok || err != nil {
		return false
	}
	return true
}

func (n *FSM) applyVariableOperation(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	var req structs.VarApplyStateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"nomad", "fsm", "apply_sv_operation"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.VarOpSet:
		return n.state.VarSet(index, &req)
	case structs.VarOpDelete:
		return n.state.VarDelete(index, &req)
	case structs.VarOpDeleteCAS:
		return n.state.VarDeleteCAS(index, &req)
	case structs.VarOpCAS:
		return n.state.VarSetCAS(index, &req)
	default:
		err := fmt.Errorf("Invalid variable operation '%s'", req.Op)
		n.logger.Warn("Invalid variable operation", "operation", req.Op)
		return err
	}
}

func (n *FSM) applyRootKeyMetaUpsert(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_root_key_meta_upsert"}, time.Now())

	var req structs.KeyringUpdateRootKeyMetaRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertRootKeyMeta(index, req.RootKeyMeta, req.Rekey); err != nil {
		n.logger.Error("UpsertRootKeyMeta failed", "error", err)
		return err
	}

	return nil
}

func (n *FSM) applyRootKeyMetaDelete(msgType structs.MessageType, buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_root_key_meta_delete"}, time.Now())

	var req structs.KeyringDeleteRootKeyRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteRootKeyMeta(index, req.KeyID); err != nil {
		n.logger.Error("DeleteRootKeyMeta failed", "error", err)
		return err
	}

	return nil
}
