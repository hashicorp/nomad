package nomad

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
)

const (
	// failedEvalUnblockInterval is the interval at which failed evaluations are
	// unblocked to re-enter the scheduler. A failed evaluation occurs under
	// high contention when the schedulers plan does not make progress.
	failedEvalUnblockInterval = 1 * time.Minute

	// replicationRateLimit is used to rate limit how often data is replicated
	// between the authoritative region and the local region
	replicationRateLimit rate.Limit = 10.0

	// barrierWriteTimeout is used to give Raft a chance to process a
	// possible loss of leadership event if we are unable to get a barrier
	// while leader.
	barrierWriteTimeout = 2 * time.Minute
)

var minAutopilotVersion = version.Must(version.NewVersion("0.8.0"))

var minSchedulerConfigVersion = version.Must(version.NewVersion("0.9.0"))

var minClusterIDVersion = version.Must(version.NewVersion("0.10.4"))

// monitorLeadership is used to monitor if we acquire or lose our role
// as the leader in the Raft cluster. There is some work the leader is
// expected to do, so we must react to changes
func (s *Server) monitorLeadership() {
	var weAreLeaderCh chan struct{}
	var leaderLoop sync.WaitGroup

	leaderCh := s.raft.LeaderCh()

	leaderStep := func(isLeader bool) {
		if isLeader {
			if weAreLeaderCh != nil {
				s.logger.Error("attempted to start the leader loop while running")
				return
			}

			weAreLeaderCh = make(chan struct{})
			leaderLoop.Add(1)
			go func(ch chan struct{}) {
				defer leaderLoop.Done()
				s.leaderLoop(ch)
			}(weAreLeaderCh)
			s.logger.Info("cluster leadership acquired")
			return
		}

		if weAreLeaderCh == nil {
			s.logger.Error("attempted to stop the leader loop while not running")
			return
		}

		s.logger.Debug("shutting down leader loop")
		close(weAreLeaderCh)
		leaderLoop.Wait()
		weAreLeaderCh = nil
		s.logger.Info("cluster leadership lost")
	}

	wasLeader := false
	for {
		select {
		case isLeader := <-leaderCh:
			if wasLeader != isLeader {
				wasLeader = isLeader
				// normal case where we went through a transition
				leaderStep(isLeader)
			} else if wasLeader && isLeader {
				// Server lost but then gained leadership immediately.
				// During this time, this server may have received
				// Raft transitions that haven't been applied to the FSM
				// yet.
				// Ensure that that FSM caught up and eval queues are refreshed
				s.logger.Warn("cluster leadership lost and gained leadership immediately.  Could indicate network issues, memory paging, or high CPU load.")

				leaderStep(false)
				leaderStep(true)
			} else {
				// Server gained but lost leadership immediately
				// before it reacted; nothing to do, move on
				s.logger.Warn("cluster leadership gained and lost leadership immediately.  Could indicate network issues, memory paging, or high CPU load.")
			}
		case <-s.shutdownCh:
			return
		}
	}
}

// leaderLoop runs as long as we are the leader to run various
// maintenance activities
func (s *Server) leaderLoop(stopCh chan struct{}) {
	var reconcileCh chan serf.Member
	establishedLeader := false

RECONCILE:
	// Setup a reconciliation timer
	reconcileCh = nil
	interval := time.After(s.config.ReconcileInterval)

	// Apply a raft barrier to ensure our FSM is caught up
	start := time.Now()
	barrier := s.raft.Barrier(barrierWriteTimeout)
	if err := barrier.Error(); err != nil {
		s.logger.Error("failed to wait for barrier", "error", err)
		goto WAIT
	}
	metrics.MeasureSince([]string{"nomad", "leader", "barrier"}, start)

	// Check if we need to handle initial leadership actions
	if !establishedLeader {
		if err := s.establishLeadership(stopCh); err != nil {
			s.logger.Error("failed to establish leadership", "error", err)

			// Immediately revoke leadership since we didn't successfully
			// establish leadership.
			if err := s.revokeLeadership(); err != nil {
				s.logger.Error("failed to revoke leadership", "error", err)
			}

			goto WAIT
		}

		establishedLeader = true
		defer func() {
			if err := s.revokeLeadership(); err != nil {
				s.logger.Error("failed to revoke leadership", "error", err)
			}
		}()
	}

	// Reconcile any missing data
	if err := s.reconcile(); err != nil {
		s.logger.Error("failed to reconcile", "error", err)
		goto WAIT
	}

	// Initial reconcile worked, now we can process the channel
	// updates
	reconcileCh = s.reconcileCh

	// Poll the stop channel to give it priority so we don't waste time
	// trying to perform the other operations if we have been asked to shut
	// down.
	select {
	case <-stopCh:
		return
	default:
	}

WAIT:
	// Wait until leadership is lost
	for {
		select {
		case <-stopCh:
			return
		case <-s.shutdownCh:
			return
		case <-interval:
			goto RECONCILE
		case member := <-reconcileCh:
			s.reconcileMember(member)
		}
	}
}

// establishLeadership is invoked once we become leader and are able
// to invoke an initial barrier. The barrier is used to ensure any
// previously inflight transactions have been committed and that our
// state is up-to-date.
func (s *Server) establishLeadership(stopCh chan struct{}) error {
	defer metrics.MeasureSince([]string{"nomad", "leader", "establish_leadership"}, time.Now())

	// Generate a leader ACL token. This will allow the leader to issue work
	// that requires a valid ACL token.
	s.setLeaderAcl(uuid.Generate())

	// Disable workers to free half the cores for use in the plan queue and
	// evaluation broker
	if numWorkers := len(s.workers); numWorkers > 1 {
		// Disabling 3/4 of the workers frees CPU for raft and the
		// plan applier which uses 1/2 the cores.
		for i := 0; i < (3 * numWorkers / 4); i++ {
			s.workers[i].SetPause(true)
		}
	}

	// Initialize and start the autopilot routine
	s.getOrCreateAutopilotConfig()
	s.autopilot.Start()

	// Initialize scheduler configuration
	s.getOrCreateSchedulerConfig()

	// Initialize the ClusterID
	_, _ = s.ClusterID()
	// todo: use cluster ID for stuff, later!

	// Enable the plan queue, since we are now the leader
	s.planQueue.SetEnabled(true)

	// Start the plan evaluator
	go s.planApply()

	// Enable the eval broker, since we are now the leader
	s.evalBroker.SetEnabled(true)

	// Enable the blocked eval tracker, since we are now the leader
	s.blockedEvals.SetEnabled(true)
	s.blockedEvals.SetTimetable(s.fsm.TimeTable())

	// Enable the deployment watcher, since we are now the leader
	s.deploymentWatcher.SetEnabled(true, s.State())

	// Enable the NodeDrainer
	s.nodeDrainer.SetEnabled(true, s.State())

	// Enable the volume watcher, since we are now the leader
	s.volumeWatcher.SetEnabled(true, s.State())

	// Restore the eval broker state
	if err := s.restoreEvals(); err != nil {
		return err
	}

	// Activate the vault client
	s.vault.SetActive(true)
	// Cleanup orphaned Vault token accessors
	if err := s.revokeVaultAccessorsOnRestore(); err != nil {
		return err
	}

	// Cleanup orphaned Service Identity token accessors
	if err := s.revokeSITokenAccessorsOnRestore(); err != nil {
		return err
	}

	// Enable the periodic dispatcher, since we are now the leader.
	s.periodicDispatcher.SetEnabled(true)

	// Restore the periodic dispatcher state
	if err := s.restorePeriodicDispatcher(); err != nil {
		return err
	}

	// Scheduler periodic jobs
	go s.schedulePeriodic(stopCh)

	// Reap any failed evaluations
	go s.reapFailedEvaluations(stopCh)

	// Reap any duplicate blocked evaluations
	go s.reapDupBlockedEvaluations(stopCh)

	// Periodically unblock failed allocations
	go s.periodicUnblockFailedEvals(stopCh)

	// Periodically publish job summary metrics
	go s.publishJobSummaryMetrics(stopCh)

	// Periodically publish job status metrics
	go s.publishJobStatusMetrics(stopCh)

	// Setup the heartbeat timers. This is done both when starting up or when
	// a leader fail over happens. Since the timers are maintained by the leader
	// node, effectively this means all the timers are renewed at the time of failover.
	// The TTL contract is that the session will not be expired before the TTL,
	// so expiring it later is allowable.
	//
	// This MUST be done after the initial barrier to ensure the latest Nodes
	// are available to be initialized. Otherwise initialization may use stale
	// data.
	if err := s.initializeHeartbeatTimers(); err != nil {
		s.logger.Error("heartbeat timer setup failed", "error", err)
		return err
	}

	// Start replication of ACLs and Policies if they are enabled,
	// and we are not the authoritative region.
	if s.config.ACLEnabled && s.config.Region != s.config.AuthoritativeRegion {
		go s.replicateACLPolicies(stopCh)
		go s.replicateACLTokens(stopCh)
	}

	// Setup any enterprise systems required.
	if err := s.establishEnterpriseLeadership(stopCh); err != nil {
		return err
	}

	s.setConsistentReadReady()

	return nil
}

// restoreEvals is used to restore pending evaluations into the eval broker and
// blocked evaluations into the blocked eval tracker. The broker and blocked
// eval tracker is maintained only by the leader, so it must be restored anytime
// a leadership transition takes place.
func (s *Server) restoreEvals() error {
	// Get an iterator over every evaluation
	ws := memdb.NewWatchSet()
	iter, err := s.fsm.State().Evals(ws)
	if err != nil {
		return fmt.Errorf("failed to get evaluations: %v", err)
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		eval := raw.(*structs.Evaluation)

		if eval.ShouldEnqueue() {
			s.evalBroker.Enqueue(eval)
		} else if eval.ShouldBlock() {
			s.blockedEvals.Block(eval)
		}
	}
	return nil
}

// revokeVaultAccessorsOnRestore is used to restore Vault accessors that should be
// revoked.
func (s *Server) revokeVaultAccessorsOnRestore() error {
	// An accessor should be revoked if its allocation or node is terminal
	ws := memdb.NewWatchSet()
	state := s.fsm.State()
	iter, err := state.VaultAccessors(ws)
	if err != nil {
		return fmt.Errorf("failed to get vault accessors: %v", err)
	}

	var revoke []*structs.VaultAccessor
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		va := raw.(*structs.VaultAccessor)

		// Check the allocation
		alloc, err := state.AllocByID(ws, va.AllocID)
		if err != nil {
			return fmt.Errorf("failed to lookup allocation %q: %v", va.AllocID, err)
		}
		if alloc == nil || alloc.Terminated() {
			// No longer running and should be revoked
			revoke = append(revoke, va)
			continue
		}

		// Check the node
		node, err := state.NodeByID(ws, va.NodeID)
		if err != nil {
			return fmt.Errorf("failed to lookup node %q: %v", va.NodeID, err)
		}
		if node == nil || node.TerminalStatus() {
			// Node is terminal so any accessor from it should be revoked
			revoke = append(revoke, va)
			continue
		}
	}

	if len(revoke) != 0 {
		if err := s.vault.RevokeTokens(context.Background(), revoke, true); err != nil {
			return fmt.Errorf("failed to revoke tokens: %v", err)
		}
	}

	return nil
}

// revokeSITokenAccessorsOnRestore is used to revoke Service Identity token
// accessors on behalf of allocs that are now gone / terminal.
func (s *Server) revokeSITokenAccessorsOnRestore() error {
	ws := memdb.NewWatchSet()
	fsmState := s.fsm.State()
	iter, err := fsmState.SITokenAccessors(ws)
	if err != nil {
		return errors.Wrap(err, "failed to get SI token accessors")
	}

	var toRevoke []*structs.SITokenAccessor
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		accessor := raw.(*structs.SITokenAccessor)

		// Check the allocation
		alloc, err := fsmState.AllocByID(ws, accessor.AllocID)
		if err != nil {
			return errors.Wrapf(err, "failed to lookup alloc %q", accessor.AllocID)
		}
		if alloc == nil || alloc.Terminated() {
			// no longer running and associated accessors should be revoked
			toRevoke = append(toRevoke, accessor)
			continue
		}

		// Check the node
		node, err := fsmState.NodeByID(ws, accessor.NodeID)
		if err != nil {
			return errors.Wrapf(err, "failed to lookup node %q", accessor.NodeID)
		}
		if node == nil || node.TerminalStatus() {
			// node is terminal and associated accessors should be revoked
			toRevoke = append(toRevoke, accessor)
			continue
		}
	}

	if len(toRevoke) > 0 {
		ctx := context.Background()
		s.consulACLs.RevokeTokens(ctx, toRevoke, true)
	}

	return nil
}

// restorePeriodicDispatcher is used to restore all periodic jobs into the
// periodic dispatcher. It also determines if a periodic job should have been
// created during the leadership transition and force runs them. The periodic
// dispatcher is maintained only by the leader, so it must be restored anytime a
// leadership transition takes place.
func (s *Server) restorePeriodicDispatcher() error {
	logger := s.logger.Named("periodic")
	ws := memdb.NewWatchSet()
	iter, err := s.fsm.State().JobsByPeriodic(ws, true)
	if err != nil {
		return fmt.Errorf("failed to get periodic jobs: %v", err)
	}

	now := time.Now()
	for i := iter.Next(); i != nil; i = iter.Next() {
		job := i.(*structs.Job)

		// We skip adding parameterized jobs because they themselves aren't
		// tracked, only the dispatched children are.
		if job.IsParameterized() {
			continue
		}

		if err := s.periodicDispatcher.Add(job); err != nil {
			logger.Error("failed to add job to periodic dispatcher", "error", err)
			continue
		}

		// We do not need to force run the job since it isn't active.
		if !job.IsPeriodicActive() {
			continue
		}

		// If the periodic job has never been launched before, launch will hold
		// the time the periodic job was added. Otherwise it has the last launch
		// time of the periodic job.
		launch, err := s.fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
		if err != nil {
			return fmt.Errorf("failed to get periodic launch time: %v", err)
		}
		if launch == nil {
			return fmt.Errorf("no recorded periodic launch time for job %q in namespace %q",
				job.ID, job.Namespace)
		}

		// nextLaunch is the next launch that should occur.
		nextLaunch, err := job.Periodic.Next(launch.Launch.In(job.Periodic.GetLocation()))
		if err != nil {
			logger.Error("failed to determine next periodic launch for job", "job", job.NamespacedID(), "error", err)
			continue
		}

		// We skip force launching the job if  there should be no next launch
		// (the zero case) or if the next launch time is in the future. If it is
		// in the future, it will be handled by the periodic dispatcher.
		if nextLaunch.IsZero() || !nextLaunch.Before(now) {
			continue
		}

		if _, err := s.periodicDispatcher.ForceRun(job.Namespace, job.ID); err != nil {
			logger.Error("force run of periodic job failed", "job", job.NamespacedID(), "error", err)
			return fmt.Errorf("force run of periodic job %q failed: %v", job.NamespacedID(), err)
		}
		logger.Debug("periodic job force runned during leadership establishment", "job", job.NamespacedID())
	}

	return nil
}

// schedulePeriodic is used to do periodic job dispatch while we are leader
func (s *Server) schedulePeriodic(stopCh chan struct{}) {
	evalGC := time.NewTicker(s.config.EvalGCInterval)
	defer evalGC.Stop()
	nodeGC := time.NewTicker(s.config.NodeGCInterval)
	defer nodeGC.Stop()
	jobGC := time.NewTicker(s.config.JobGCInterval)
	defer jobGC.Stop()
	deploymentGC := time.NewTicker(s.config.DeploymentGCInterval)
	defer deploymentGC.Stop()
	csiPluginGC := time.NewTicker(s.config.CSIPluginGCInterval)
	defer csiPluginGC.Stop()
	csiVolumeClaimGC := time.NewTicker(s.config.CSIVolumeClaimGCInterval)
	defer csiVolumeClaimGC.Stop()

	// getLatest grabs the latest index from the state store. It returns true if
	// the index was retrieved successfully.
	getLatest := func() (uint64, bool) {
		snapshotIndex, err := s.fsm.State().LatestIndex()
		if err != nil {
			s.logger.Error("failed to determine state store's index", "error", err)
			return 0, false
		}

		return snapshotIndex, true
	}

	for {

		select {
		case <-evalGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobEvalGC, index))
			}
		case <-nodeGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobNodeGC, index))
			}
		case <-jobGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobJobGC, index))
			}
		case <-deploymentGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobDeploymentGC, index))
			}
		case <-csiPluginGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobCSIPluginGC, index))
			}
		case <-csiVolumeClaimGC.C:
			if index, ok := getLatest(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobCSIVolumeClaimGC, index))
			}

		case <-stopCh:
			return
		}
	}
}

// coreJobEval returns an evaluation for a core job
func (s *Server) coreJobEval(job string, modifyIndex uint64) *structs.Evaluation {
	return &structs.Evaluation{
		ID:          uuid.Generate(),
		Namespace:   "-",
		Priority:    structs.CoreJobPriority,
		Type:        structs.JobTypeCore,
		TriggeredBy: structs.EvalTriggerScheduled,
		JobID:       job,
		LeaderACL:   s.getLeaderAcl(),
		Status:      structs.EvalStatusPending,
		ModifyIndex: modifyIndex,
	}
}

// reapFailedEvaluations is used to reap evaluations that
// have reached their delivery limit and should be failed
func (s *Server) reapFailedEvaluations(stopCh chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
			// Scan for a failed evaluation
			eval, token, err := s.evalBroker.Dequeue([]string{failedQueue}, time.Second)
			if err != nil {
				return
			}
			if eval == nil {
				continue
			}

			// Update the status to failed
			updateEval := eval.Copy()
			updateEval.Status = structs.EvalStatusFailed
			updateEval.StatusDescription = fmt.Sprintf("evaluation reached delivery limit (%d)", s.config.EvalDeliveryLimit)
			s.logger.Warn("eval reached delivery limit, marking as failed", "eval", updateEval.GoString())

			// Create a follow-up evaluation that will be used to retry the
			// scheduling for the job after the cluster is hopefully more stable
			// due to the fairly large backoff.
			followupEvalWait := s.config.EvalFailedFollowupBaselineDelay +
				time.Duration(rand.Int63n(int64(s.config.EvalFailedFollowupDelayRange)))

			followupEval := eval.CreateFailedFollowUpEval(followupEvalWait)
			updateEval.NextEval = followupEval.ID
			updateEval.UpdateModifyTime()

			// Update via Raft
			req := structs.EvalUpdateRequest{
				Evals: []*structs.Evaluation{updateEval, followupEval},
			}
			if _, _, err := s.raftApply(structs.EvalUpdateRequestType, &req); err != nil {
				s.logger.Error("failed to update failed eval and create a follow-up", "eval", updateEval.GoString(), "error", err)
				continue
			}

			// Ack completion
			s.evalBroker.Ack(eval.ID, token)
		}
	}
}

// reapDupBlockedEvaluations is used to reap duplicate blocked evaluations and
// should be cancelled.
func (s *Server) reapDupBlockedEvaluations(stopCh chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
			// Scan for duplicate blocked evals.
			dups := s.blockedEvals.GetDuplicates(time.Second)
			if dups == nil {
				continue
			}

			cancel := make([]*structs.Evaluation, len(dups))
			for i, dup := range dups {
				// Update the status to cancelled
				newEval := dup.Copy()
				newEval.Status = structs.EvalStatusCancelled
				newEval.StatusDescription = fmt.Sprintf("existing blocked evaluation exists for job %q", newEval.JobID)
				newEval.UpdateModifyTime()
				cancel[i] = newEval
			}

			// Update via Raft
			req := structs.EvalUpdateRequest{
				Evals: cancel,
			}
			if _, _, err := s.raftApply(structs.EvalUpdateRequestType, &req); err != nil {
				s.logger.Error("failed to update duplicate evals", "evals", log.Fmt("%#v", cancel), "error", err)
				continue
			}
		}
	}
}

// periodicUnblockFailedEvals periodically unblocks failed, blocked evaluations.
func (s *Server) periodicUnblockFailedEvals(stopCh chan struct{}) {
	ticker := time.NewTicker(failedEvalUnblockInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Unblock the failed allocations
			s.blockedEvals.UnblockFailed()
		}
	}
}

// publishJobSummaryMetrics publishes the job summaries as metrics
func (s *Server) publishJobSummaryMetrics(stopCh chan struct{}) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-timer.C:
			timer.Reset(s.config.StatsCollectionInterval)
			state, err := s.State().Snapshot()
			if err != nil {
				s.logger.Error("failed to get state", "error", err)
				continue
			}
			ws := memdb.NewWatchSet()
			iter, err := state.JobSummaries(ws)
			if err != nil {
				s.logger.Error("failed to get job summaries", "error", err)
				continue
			}

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				summary := raw.(*structs.JobSummary)
				if s.config.DisableDispatchedJobSummaryMetrics {
					job, err := state.JobByID(ws, summary.Namespace, summary.JobID)
					if err != nil {
						s.logger.Error("error getting job for summary", "error", err)
						continue
					}
					if job.Dispatched {
						continue
					}
				}
				s.iterateJobSummaryMetrics(summary)
			}
		}
	}
}

func (s *Server) iterateJobSummaryMetrics(summary *structs.JobSummary) {
	for name, tgSummary := range summary.Summary {
		if !s.config.DisableTaggedMetrics {
			labels := []metrics.Label{
				{
					Name:  "job",
					Value: summary.JobID,
				},
				{
					Name:  "task_group",
					Value: name,
				},
				{
					Name:  "namespace",
					Value: summary.Namespace,
				},
			}

			if strings.Contains(summary.JobID, "/dispatch-") {
				jobInfo := strings.Split(summary.JobID, "/dispatch-")
				labels = append(labels, metrics.Label{
					Name:  "parent_id",
					Value: jobInfo[0],
				}, metrics.Label{
					Name:  "dispatch_id",
					Value: jobInfo[1],
				})
			}

			if strings.Contains(summary.JobID, "/periodic-") {
				jobInfo := strings.Split(summary.JobID, "/periodic-")
				labels = append(labels, metrics.Label{
					Name:  "parent_id",
					Value: jobInfo[0],
				}, metrics.Label{
					Name:  "periodic_id",
					Value: jobInfo[1],
				})
			}

			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "queued"},
				float32(tgSummary.Queued), labels)
			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "complete"},
				float32(tgSummary.Complete), labels)
			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "failed"},
				float32(tgSummary.Failed), labels)
			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "running"},
				float32(tgSummary.Running), labels)
			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "starting"},
				float32(tgSummary.Starting), labels)
			metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "lost"},
				float32(tgSummary.Lost), labels)
		}
		if s.config.BackwardsCompatibleMetrics {
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "queued"}, float32(tgSummary.Queued))
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "complete"}, float32(tgSummary.Complete))
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "failed"}, float32(tgSummary.Failed))
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "running"}, float32(tgSummary.Running))
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "starting"}, float32(tgSummary.Starting))
			metrics.SetGauge([]string{"nomad", "job_summary", summary.JobID, name, "lost"}, float32(tgSummary.Lost))
		}
	}
}

// publishJobStatusMetrics publishes the job statuses as metrics
func (s *Server) publishJobStatusMetrics(stopCh chan struct{}) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-timer.C:
			timer.Reset(s.config.StatsCollectionInterval)
			state, err := s.State().Snapshot()
			if err != nil {
				s.logger.Error("failed to get state", "error", err)
				continue
			}
			ws := memdb.NewWatchSet()
			iter, err := state.Jobs(ws)
			if err != nil {
				s.logger.Error("failed to get job statuses", "error", err)
				continue
			}

			s.iterateJobStatusMetrics(&iter)
		}
	}
}

func (s *Server) iterateJobStatusMetrics(jobs *memdb.ResultIterator) {
	var pending int64 // Sum of all jobs in 'pending' state
	var running int64 // Sum of all jobs in 'running' state
	var dead int64    // Sum of all jobs in 'dead' state

	for {
		raw := (*jobs).Next()
		if raw == nil {
			break
		}

		job := raw.(*structs.Job)

		switch job.Status {
		case structs.JobStatusPending:
			pending++
		case structs.JobStatusRunning:
			running++
		case structs.JobStatusDead:
			dead++
		}
	}

	metrics.SetGauge([]string{"nomad", "job_status", "pending"}, float32(pending))
	metrics.SetGauge([]string{"nomad", "job_status", "running"}, float32(running))
	metrics.SetGauge([]string{"nomad", "job_status", "dead"}, float32(dead))
}

// revokeLeadership is invoked once we step down as leader.
// This is used to cleanup any state that may be specific to a leader.
func (s *Server) revokeLeadership() error {
	defer metrics.MeasureSince([]string{"nomad", "leader", "revoke_leadership"}, time.Now())

	s.resetConsistentReadReady()

	// Clear the leader token since we are no longer the leader.
	s.setLeaderAcl("")

	// Disable autopilot
	s.autopilot.Stop()

	// Disable the plan queue, since we are no longer leader
	s.planQueue.SetEnabled(false)

	// Disable the eval broker, since it is only useful as a leader
	s.evalBroker.SetEnabled(false)

	// Disable the blocked eval tracker, since it is only useful as a leader
	s.blockedEvals.SetEnabled(false)

	// Disable the periodic dispatcher, since it is only useful as a leader
	s.periodicDispatcher.SetEnabled(false)

	// Disable the Vault client as it is only useful as a leader.
	s.vault.SetActive(false)

	// Disable the deployment watcher as it is only useful as a leader.
	s.deploymentWatcher.SetEnabled(false, nil)

	// Disable the node drainer
	s.nodeDrainer.SetEnabled(false, nil)

	// Disable the volume watcher
	s.volumeWatcher.SetEnabled(false, nil)

	// Disable any enterprise systems required.
	if err := s.revokeEnterpriseLeadership(); err != nil {
		return err
	}

	// Clear the heartbeat timers on either shutdown or step down,
	// since we are no longer responsible for TTL expirations.
	if err := s.clearAllHeartbeatTimers(); err != nil {
		s.logger.Error("clearing heartbeat timers failed", "error", err)
		return err
	}

	// Unpause our worker if we paused previously
	if len(s.workers) > 1 {
		for i := 0; i < len(s.workers)/2; i++ {
			s.workers[i].SetPause(false)
		}
	}
	return nil
}

// reconcile is used to reconcile the differences between Serf
// membership and what is reflected in our strongly consistent store.
func (s *Server) reconcile() error {
	defer metrics.MeasureSince([]string{"nomad", "leader", "reconcile"}, time.Now())
	members := s.serf.Members()
	for _, member := range members {
		if err := s.reconcileMember(member); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMember is used to do an async reconcile of a single serf member
func (s *Server) reconcileMember(member serf.Member) error {
	// Check if this is a member we should handle
	valid, parts := isNomadServer(member)
	if !valid || parts.Region != s.config.Region {
		return nil
	}
	defer metrics.MeasureSince([]string{"nomad", "leader", "reconcileMember"}, time.Now())

	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.addRaftPeer(member, parts)
	case serf.StatusLeft, StatusReap:
		err = s.removeRaftPeer(member, parts)
	}
	if err != nil {
		s.logger.Error("failed to reconcile member", "member", member, "error", err)
		return err
	}
	return nil
}

// addRaftPeer is used to add a new Raft peer when a Nomad server joins
func (s *Server) addRaftPeer(m serf.Member, parts *serverParts) error {
	// Check for possibility of multiple bootstrap nodes
	members := s.serf.Members()
	if parts.Bootstrap {
		for _, member := range members {
			valid, p := isNomadServer(member)
			if valid && member.Name != m.Name && p.Bootstrap {
				s.logger.Error("skipping adding Raft peer because an existing peer is in bootstrap mode and only one server should be in bootstrap mode",
					"existing_peer", member.Name, "joining_peer", m.Name)
				return nil
			}
		}
	}

	// Processing ourselves could result in trying to remove ourselves to
	// fix up our address, which would make us step down. This is only
	// safe to attempt if there are multiple servers available.
	addr := (&net.TCPAddr{IP: m.Addr, Port: parts.Port}).String()
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error("failed to get raft configuration", "error", err)
		return err
	}

	if m.Name == s.config.NodeName {
		if l := len(configFuture.Configuration().Servers); l < 3 {
			s.logger.Debug("skipping self join check for peer since the cluster is too small", "peer", m.Name)
			return nil
		}
	}

	// See if it's already in the configuration. It's harmless to re-add it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries. If the address is the same but the ID changed, remove the
	// old server before adding the new one.
	minRaftProtocol, err := s.autopilot.MinRaftProtocol()
	if err != nil {
		return err
	}
	for _, server := range configFuture.Configuration().Servers {
		// No-op if the raft version is too low
		if server.Address == raft.ServerAddress(addr) && (minRaftProtocol < 2 || parts.RaftVersion < 3) {
			return nil
		}

		// If the address or ID matches an existing server, see if we need to remove the old one first
		if server.Address == raft.ServerAddress(addr) || server.ID == raft.ServerID(parts.ID) {
			// Exit with no-op if this is being called on an existing server and both the ID and address match
			if server.Address == raft.ServerAddress(addr) && server.ID == raft.ServerID(parts.ID) {
				return nil
			}
			future := s.raft.RemoveServer(server.ID, 0, 0)
			if server.Address == raft.ServerAddress(addr) {
				if err := future.Error(); err != nil {
					return fmt.Errorf("error removing server with duplicate address %q: %s", server.Address, err)
				}
				s.logger.Info("removed server with duplicate address", "address", server.Address)
			} else {
				if err := future.Error(); err != nil {
					return fmt.Errorf("error removing server with duplicate ID %q: %s", server.ID, err)
				}
				s.logger.Info("removed server with duplicate ID", "id", server.ID)
			}
		}
	}

	// Attempt to add as a peer
	switch {
	case minRaftProtocol >= 3:
		addFuture := s.raft.AddNonvoter(raft.ServerID(parts.ID), raft.ServerAddress(addr), 0, 0)
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	case minRaftProtocol == 2 && parts.RaftVersion >= 3:
		addFuture := s.raft.AddVoter(raft.ServerID(parts.ID), raft.ServerAddress(addr), 0, 0)
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	default:
		addFuture := s.raft.AddPeer(raft.ServerAddress(addr))
		if err := addFuture.Error(); err != nil {
			s.logger.Error("failed to add raft peer", "error", err)
			return err
		}
	}

	return nil
}

// removeRaftPeer is used to remove a Raft peer when a Nomad server leaves
// or is reaped
func (s *Server) removeRaftPeer(m serf.Member, parts *serverParts) error {
	addr := (&net.TCPAddr{IP: m.Addr, Port: parts.Port}).String()

	// See if it's already in the configuration. It's harmless to re-remove it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries.
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error("failed to get raft configuration", "error", err)
		return err
	}

	minRaftProtocol, err := s.autopilot.MinRaftProtocol()
	if err != nil {
		return err
	}

	// Pick which remove API to use based on how the server was added.
	for _, server := range configFuture.Configuration().Servers {
		// If we understand the new add/remove APIs and the server was added by ID, use the new remove API
		if minRaftProtocol >= 2 && server.ID == raft.ServerID(parts.ID) {
			s.logger.Info("removing server by ID", "id", server.ID)
			future := s.raft.RemoveServer(raft.ServerID(parts.ID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove raft peer", "id", server.ID, "error", err)
				return err
			}
			break
		} else if server.Address == raft.ServerAddress(addr) {
			// If not, use the old remove API
			s.logger.Info("removing server by address", "address", server.Address)
			future := s.raft.RemovePeer(raft.ServerAddress(addr))
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove raft peer", "address", addr, "error", err)
				return err
			}
			break
		}
	}

	return nil
}

// replicateACLPolicies is used to replicate ACL policies from
// the authoritative region to this region.
func (s *Server) replicateACLPolicies(stopCh chan struct{}) {
	req := structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting ACL policy replication from authoritative region", "authoritative_region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
			// Rate limit how often we attempt replication
			limiter.Wait(context.Background())

			// Fetch the list of policies
			var resp structs.ACLPolicyListResponse
			req.AuthToken = s.ReplicationToken()
			err := s.forwardRegion(s.config.AuthoritativeRegion,
				"ACL.ListPolicies", &req, &resp)
			if err != nil {
				s.logger.Error("failed to fetch policies from authoritative region", "error", err)
				goto ERR_WAIT
			}

			// Perform a two-way diff
			delete, update := diffACLPolicies(s.State(), req.MinQueryIndex, resp.Policies)

			// Delete policies that should not exist
			if len(delete) > 0 {
				args := &structs.ACLPolicyDeleteRequest{
					Names: delete,
				}
				_, _, err := s.raftApply(structs.ACLPolicyDeleteRequestType, args)
				if err != nil {
					s.logger.Error("failed to delete policies", "error", err)
					goto ERR_WAIT
				}
			}

			// Fetch any outdated policies
			var fetched []*structs.ACLPolicy
			if len(update) > 0 {
				req := structs.ACLPolicySetRequest{
					Names: update,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						AuthToken:     s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.ACLPolicySetResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion,
					"ACL.GetPolicies", &req, &reply); err != nil {
					s.logger.Error("failed to fetch policies from authoritative region", "error", err)
					goto ERR_WAIT
				}
				for _, policy := range reply.Policies {
					fetched = append(fetched, policy)
				}
			}

			// Update local policies
			if len(fetched) > 0 {
				args := &structs.ACLPolicyUpsertRequest{
					Policies: fetched,
				}
				_, _, err := s.raftApply(structs.ACLPolicyUpsertRequestType, args)
				if err != nil {
					s.logger.Error("failed to update policies", "error", err)
					goto ERR_WAIT
				}
			}

			// Update the minimum query index, blocks until there
			// is a change.
			req.MinQueryIndex = resp.Index
		}
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

// diffACLPolicies is used to perform a two-way diff between the local
// policies and the remote policies to determine which policies need to
// be deleted or updated.
func diffACLPolicies(state *state.StateStore, minIndex uint64, remoteList []*structs.ACLPolicyListStub) (delete []string, update []string) {
	// Construct a set of the local and remote policies
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local policies
	iter, err := state.ACLPolicies(nil)
	if err != nil {
		panic("failed to iterate local policies")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policy := raw.(*structs.ACLPolicy)
		local[policy.Name] = policy.Hash
	}

	// Iterate over the remote policies
	for _, rp := range remoteList {
		remote[rp.Name] = struct{}{}

		// Check if the policy is missing locally
		if localHash, ok := local[rp.Name]; !ok {
			update = append(update, rp.Name)

			// Check if policy is newer remotely and there is a hash mis-match.
		} else if rp.ModifyIndex > minIndex && !bytes.Equal(localHash, rp.Hash) {
			update = append(update, rp.Name)
		}
	}

	// Check if policy should be deleted
	for lp := range local {
		if _, ok := remote[lp]; !ok {
			delete = append(delete, lp)
		}
	}
	return
}

// replicateACLTokens is used to replicate global ACL tokens from
// the authoritative region to this region.
func (s *Server) replicateACLTokens(stopCh chan struct{}) {
	req := structs.ACLTokenListRequest{
		GlobalOnly: true,
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting ACL token replication from authoritative region", "authoritative_region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
			// Rate limit how often we attempt replication
			limiter.Wait(context.Background())

			// Fetch the list of tokens
			var resp structs.ACLTokenListResponse
			req.AuthToken = s.ReplicationToken()
			err := s.forwardRegion(s.config.AuthoritativeRegion,
				"ACL.ListTokens", &req, &resp)
			if err != nil {
				s.logger.Error("failed to fetch tokens from authoritative region", "error", err)
				goto ERR_WAIT
			}

			// Perform a two-way diff
			delete, update := diffACLTokens(s.State(), req.MinQueryIndex, resp.Tokens)

			// Delete tokens that should not exist
			if len(delete) > 0 {
				args := &structs.ACLTokenDeleteRequest{
					AccessorIDs: delete,
				}
				_, _, err := s.raftApply(structs.ACLTokenDeleteRequestType, args)
				if err != nil {
					s.logger.Error("failed to delete tokens", "error", err)
					goto ERR_WAIT
				}
			}

			// Fetch any outdated policies.
			var fetched []*structs.ACLToken
			if len(update) > 0 {
				req := structs.ACLTokenSetRequest{
					AccessorIDS: update,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						AuthToken:     s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.ACLTokenSetResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion,
					"ACL.GetTokens", &req, &reply); err != nil {
					s.logger.Error("failed to fetch tokens from authoritative region", "error", err)
					goto ERR_WAIT
				}
				for _, token := range reply.Tokens {
					fetched = append(fetched, token)
				}
			}

			// Update local tokens
			if len(fetched) > 0 {
				args := &structs.ACLTokenUpsertRequest{
					Tokens: fetched,
				}
				_, _, err := s.raftApply(structs.ACLTokenUpsertRequestType, args)
				if err != nil {
					s.logger.Error("failed to update tokens", "error", err)
					goto ERR_WAIT
				}
			}

			// Update the minimum query index, blocks until there
			// is a change.
			req.MinQueryIndex = resp.Index
		}
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

// diffACLTokens is used to perform a two-way diff between the local
// tokens and the remote tokens to determine which tokens need to
// be deleted or updated.
func diffACLTokens(state *state.StateStore, minIndex uint64, remoteList []*structs.ACLTokenListStub) (delete []string, update []string) {
	// Construct a set of the local and remote policies
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local global tokens
	iter, err := state.ACLTokensByGlobal(nil, true)
	if err != nil {
		panic("failed to iterate local tokens")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		token := raw.(*structs.ACLToken)
		local[token.AccessorID] = token.Hash
	}

	// Iterate over the remote tokens
	for _, rp := range remoteList {
		remote[rp.AccessorID] = struct{}{}

		// Check if the token is missing locally
		if localHash, ok := local[rp.AccessorID]; !ok {
			update = append(update, rp.AccessorID)

			// Check if policy is newer remotely and there is a hash mis-match.
		} else if rp.ModifyIndex > minIndex && !bytes.Equal(localHash, rp.Hash) {
			update = append(update, rp.AccessorID)
		}
	}

	// Check if local token should be deleted
	for lp := range local {
		if _, ok := remote[lp]; !ok {
			delete = append(delete, lp)
		}
	}
	return
}

// getOrCreateAutopilotConfig is used to get the autopilot config, initializing it if necessary
func (s *Server) getOrCreateAutopilotConfig() *structs.AutopilotConfig {
	state := s.fsm.State()
	_, config, err := state.AutopilotConfig()
	if err != nil {
		s.logger.Named("autopilot").Error("failed to get autopilot config", "error", err)
		return nil
	}
	if config != nil {
		return config
	}

	if !ServersMeetMinimumVersion(s.Members(), minAutopilotVersion, false) {
		s.logger.Named("autopilot").Warn("can't initialize until all servers are above minimum version", "min_version", minAutopilotVersion)
		return nil
	}

	config = s.config.AutopilotConfig
	req := structs.AutopilotSetConfigRequest{Config: *config}
	if _, _, err = s.raftApply(structs.AutopilotRequestType, req); err != nil {
		s.logger.Named("autopilot").Error("failed to initialize config", "error", err)
		return nil
	}

	return config
}

// getOrCreateSchedulerConfig is used to get the scheduler config. We create a default
// config if it doesn't already exist for bootstrapping an empty cluster
func (s *Server) getOrCreateSchedulerConfig() *structs.SchedulerConfiguration {
	state := s.fsm.State()
	_, config, err := state.SchedulerConfig()
	if err != nil {
		s.logger.Named("core").Error("failed to get scheduler config", "error", err)
		return nil
	}
	if config != nil {
		return config
	}
	if !ServersMeetMinimumVersion(s.Members(), minSchedulerConfigVersion, false) {
		s.logger.Named("core").Warn("can't initialize scheduler config until all servers are above minimum version", "min_version", minSchedulerConfigVersion)
		return nil
	}

	req := structs.SchedulerSetConfigRequest{Config: s.config.DefaultSchedulerConfig}
	if _, _, err = s.raftApply(structs.SchedulerConfigRequestType, req); err != nil {
		s.logger.Named("core").Error("failed to initialize config", "error", err)
		return nil
	}

	return config
}

func (s *Server) generateClusterID() (string, error) {
	if !ServersMeetMinimumVersion(s.Members(), minClusterIDVersion, false) {
		s.logger.Named("core").Warn("cannot initialize cluster ID until all servers are above minimum version", "min_version", minClusterIDVersion)
		return "", errors.Errorf("cluster ID cannot be created until all servers are above minimum version %s", minClusterIDVersion)
	}

	newMeta := structs.ClusterMetadata{ClusterID: uuid.Generate(), CreateTime: time.Now().UnixNano()}
	if _, _, err := s.raftApply(structs.ClusterMetadataRequestType, newMeta); err != nil {
		s.logger.Named("core").Error("failed to create cluster ID", "error", err)
		return "", errors.Wrap(err, "failed to create cluster ID")
	}

	s.logger.Named("core").Info("established cluster id", "cluster_id", newMeta.ClusterID, "create_time", newMeta.CreateTime)
	return newMeta.ClusterID, nil
}
