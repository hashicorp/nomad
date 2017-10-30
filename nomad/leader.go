package nomad

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"golang.org/x/time/rate"

	"github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

const (
	// failedEvalUnblockInterval is the interval at which failed evaluations are
	// unblocked to re-enter the scheduler. A failed evaluation occurs under
	// high contention when the schedulers plan does not make progress.
	failedEvalUnblockInterval = 1 * time.Minute

	// replicationRateLimit is used to rate limit how often data is replicated
	// between the authoritative region and the local region
	replicationRateLimit rate.Limit = 10.0
)

// monitorLeadership is used to monitor if we acquire or lose our role
// as the leader in the Raft cluster. There is some work the leader is
// expected to do, so we must react to changes
func (s *Server) monitorLeadership() {
	var stopCh chan struct{}
	for {
		select {
		case isLeader := <-s.leaderCh:
			if isLeader {
				stopCh = make(chan struct{})
				go s.leaderLoop(stopCh)
				s.logger.Printf("[INFO] nomad: cluster leadership acquired")
			} else if stopCh != nil {
				close(stopCh)
				stopCh = nil
				s.logger.Printf("[INFO] nomad: cluster leadership lost")
			}
		case <-s.shutdownCh:
			return
		}
	}
}

// leaderLoop runs as long as we are the leader to run various
// maintence activities
func (s *Server) leaderLoop(stopCh chan struct{}) {
	// Ensure we revoke leadership on stepdown
	defer s.revokeLeadership()

	var reconcileCh chan serf.Member
	establishedLeader := false

RECONCILE:
	// Setup a reconciliation timer
	reconcileCh = nil
	interval := time.After(s.config.ReconcileInterval)

	// Apply a raft barrier to ensure our FSM is caught up
	start := time.Now()
	barrier := s.raft.Barrier(0)
	if err := barrier.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to wait for barrier: %v", err)
		goto WAIT
	}
	metrics.MeasureSince([]string{"nomad", "leader", "barrier"}, start)

	// Check if we need to handle initial leadership actions
	if !establishedLeader {
		if err := s.establishLeadership(stopCh); err != nil {
			s.logger.Printf("[ERR] nomad: failed to establish leadership: %v", err)
			goto WAIT
		}
		establishedLeader = true
	}

	// Reconcile any missing data
	if err := s.reconcile(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to reconcile: %v", err)
		goto WAIT
	}

	// Initial reconcile worked, now we can process the channel
	// updates
	reconcileCh = s.reconcileCh

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
	if err := s.deploymentWatcher.SetEnabled(true, s.State()); err != nil {
		return err
	}

	// Restore the eval broker state
	if err := s.restoreEvals(); err != nil {
		return err
	}

	// Activate the vault client
	s.vault.SetActive(true)
	if err := s.restoreRevokingAccessors(); err != nil {
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
		s.logger.Printf("[ERR] nomad: heartbeat timer setup failed: %v", err)
		return err
	}

	// COMPAT 0.4 - 0.4.1
	// Reconcile the summaries of the registered jobs. We reconcile summaries
	// only if the server is 0.4.1 since summaries are not present in 0.4 they
	// might be incorrect after upgrading to 0.4.1 the summaries might not be
	// correct
	if err := s.reconcileJobSummaries(); err != nil {
		return fmt.Errorf("unable to reconcile job summaries: %v", err)
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

// restoreRevokingAccessors is used to restore Vault accessors that should be
// revoked.
func (s *Server) restoreRevokingAccessors() error {
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

// restorePeriodicDispatcher is used to restore all periodic jobs into the
// periodic dispatcher. It also determines if a periodic job should have been
// created during the leadership transition and force runs them. The periodic
// dispatcher is maintained only by the leader, so it must be restored anytime a
// leadership transition takes place.
func (s *Server) restorePeriodicDispatcher() error {
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

		added, err := s.periodicDispatcher.Add(job)
		if err != nil {
			return err
		}

		// We did not add the job to the tracker, this can be for a variety of
		// reasons, but it means that we do not need to force run it.
		if !added {
			continue
		}

		// If the periodic job has never been launched before, launch will hold
		// the time the periodic job was added. Otherwise it has the last launch
		// time of the periodic job.
		launch, err := s.fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
		if err != nil || launch == nil {
			return fmt.Errorf("failed to get periodic launch time: %v", err)
		}

		// nextLaunch is the next launch that should occur.
		nextLaunch := job.Periodic.Next(launch.Launch.In(job.Periodic.GetLocation()))

		// We skip force launching the job if  there should be no next launch
		// (the zero case) or if the next launch time is in the future. If it is
		// in the future, it will be handled by the periodic dispatcher.
		if nextLaunch.IsZero() || !nextLaunch.Before(now) {
			continue
		}

		if _, err := s.periodicDispatcher.ForceRun(job.Namespace, job.ID); err != nil {
			msg := fmt.Sprintf("force run of periodic job %q failed: %v", job.ID, err)
			s.logger.Printf("[ERR] nomad.periodic: %s", msg)
			return errors.New(msg)
		}
		s.logger.Printf("[DEBUG] nomad.periodic: periodic job %q force"+
			" run during leadership establishment", job.ID)
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

	// getLatest grabs the latest index from the state store. It returns true if
	// the index was retrieved successfully.
	getLatest := func() (uint64, bool) {
		snapshotIndex, err := s.fsm.State().LatestIndex()
		if err != nil {
			s.logger.Printf("[ERR] nomad: failed to determine state store's index: %v", err)
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
			s.logger.Printf("[WARN] nomad: eval %#v reached delivery limit, marking as failed", updateEval)

			// Create a follow-up evaluation that will be used to retry the
			// scheduling for the job after the cluster is hopefully more stable
			// due to the fairly large backoff.
			followupEvalWait := s.config.EvalFailedFollowupBaselineDelay +
				time.Duration(rand.Int63n(int64(s.config.EvalFailedFollowupDelayRange)))
			followupEval := eval.CreateFailedFollowUpEval(followupEvalWait)

			// Update via Raft
			req := structs.EvalUpdateRequest{
				Evals: []*structs.Evaluation{updateEval, followupEval},
			}
			if _, _, err := s.raftApply(structs.EvalUpdateRequestType, &req); err != nil {
				s.logger.Printf("[ERR] nomad: failed to update failed eval %#v and create a follow-up: %v", updateEval, err)
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
				cancel[i] = newEval
			}

			// Update via Raft
			req := structs.EvalUpdateRequest{
				Evals: cancel,
			}
			if _, _, err := s.raftApply(structs.EvalUpdateRequestType, &req); err != nil {
				s.logger.Printf("[ERR] nomad: failed to update duplicate evals %#v: %v", cancel, err)
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
	// Using a timer instead of a ticker so that we can publish after the
	// current batch of metrics have been published
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-timer.C:
			state, err := s.State().Snapshot()
			if err != nil {
				timer.Reset(s.config.StatsCollectionInterval)
				s.logger.Printf("[ERR] nomad: failed to get state: %v", err)
				continue
			}
			ws := memdb.NewWatchSet()
			iter, err := state.JobSummaries(ws)
			if err != nil {
				timer.Reset(s.config.StatsCollectionInterval)
				s.logger.Printf("[ERR] nomad: failed to get job summaries: %v", err)
				continue
			}

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				summary := raw.(*structs.JobSummary)
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
			timer.Reset(s.config.StatsCollectionInterval)
		}
	}
}

// revokeLeadership is invoked once we step down as leader.
// This is used to cleanup any state that may be specific to a leader.
func (s *Server) revokeLeadership() error {
	// Clear the leader token since we are no longer the leader.
	s.setLeaderAcl("")

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
	if err := s.deploymentWatcher.SetEnabled(false, nil); err != nil {
		return err
	}

	// Disable any enterprise systems required.
	if err := s.revokeEnterpriseLeadership(); err != nil {
		return err
	}

	// Clear the heartbeat timers on either shutdown or step down,
	// since we are no longer responsible for TTL expirations.
	if err := s.clearAllHeartbeatTimers(); err != nil {
		s.logger.Printf("[ERR] nomad: clearing heartbeat timers failed: %v", err)
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

	// Do not reconcile ourself
	if member.Name == fmt.Sprintf("%s.%s", s.config.NodeName, s.config.Region) {
		return nil
	}

	var err error
	switch member.Status {
	case serf.StatusAlive:
		err = s.addRaftPeer(member, parts)
	case serf.StatusLeft, StatusReap:
		err = s.removeRaftPeer(member, parts)
	}
	if err != nil {
		s.logger.Printf("[ERR] nomad: failed to reconcile member: %v: %v",
			member, err)
		return err
	}
	return nil
}

// reconcileJobSummaries reconciles the summaries of all the jobs registered in
// the system
// COMPAT 0.4 -> 0.4.1
func (s *Server) reconcileJobSummaries() error {
	index, err := s.fsm.state.LatestIndex()
	if err != nil {
		return fmt.Errorf("unable to read latest index: %v", err)
	}
	s.logger.Printf("[DEBUG] leader: reconciling job summaries at index: %v", index)

	args := &structs.GenericResponse{}
	msg := structs.ReconcileJobSummariesRequestType | structs.IgnoreUnknownTypeFlag
	if _, _, err = s.raftApply(msg, args); err != nil {
		return fmt.Errorf("reconciliation of job summaries failed: %v", err)
	}

	return nil
}

// addRaftPeer is used to add a new Raft peer when a Nomad server joins
func (s *Server) addRaftPeer(m serf.Member, parts *serverParts) error {
	// Do not join ourselfs
	if m.Name == s.config.NodeName {
		s.logger.Printf("[DEBUG] nomad: adding self (%q) as raft peer skipped", m.Name)
		return nil
	}

	// Check for possibility of multiple bootstrap nodes
	if parts.Bootstrap {
		members := s.serf.Members()
		for _, member := range members {
			valid, p := isNomadServer(member)
			if valid && member.Name != m.Name && p.Bootstrap {
				s.logger.Printf("[ERR] nomad: '%v' and '%v' are both in bootstrap mode. Only one node should be in bootstrap mode, not adding Raft peer.", m.Name, member.Name)
				return nil
			}
		}
	}

	// TODO (alexdadgar) - This will need to be changed once we support node IDs.
	addr := (&net.TCPAddr{IP: m.Addr, Port: parts.Port}).String()

	// See if it's already in the configuration. It's harmless to re-add it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries.
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to get raft configuration: %v", err)
		return err
	}
	for _, server := range configFuture.Configuration().Servers {
		if server.Address == raft.ServerAddress(addr) {
			return nil
		}
	}

	// Attempt to add as a peer
	addFuture := s.raft.AddPeer(raft.ServerAddress(addr))
	if err := addFuture.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to add raft peer: %v", err)
		return err
	} else if err == nil {
		s.logger.Printf("[INFO] nomad: added raft peer: %v", parts)
	}
	return nil
}

// removeRaftPeer is used to remove a Raft peer when a Nomad server leaves
// or is reaped
func (s *Server) removeRaftPeer(m serf.Member, parts *serverParts) error {
	// TODO (alexdadgar) - This will need to be changed once we support node IDs.
	addr := (&net.TCPAddr{IP: m.Addr, Port: parts.Port}).String()

	// See if it's already in the configuration. It's harmless to re-remove it
	// but we want to avoid doing that if possible to prevent useless Raft
	// log entries.
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to get raft configuration: %v", err)
		return err
	}
	for _, server := range configFuture.Configuration().Servers {
		if server.Address == raft.ServerAddress(addr) {
			goto REMOVE
		}
	}
	return nil

REMOVE:
	// Attempt to remove as a peer.
	future := s.raft.RemovePeer(raft.ServerAddress(addr))
	if err := future.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: failed to remove raft peer '%v': %v",
			parts, err)
		return err
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
	s.logger.Printf("[DEBUG] nomad: starting ACL policy replication from authoritative region %q", req.Region)

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
				s.logger.Printf("[ERR] nomad: failed to fetch policies from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to delete policies: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to fetch policies from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to update policies: %v", err)
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
	s.logger.Printf("[DEBUG] nomad: starting ACL token replication from authoritative region %q", req.Region)

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
				s.logger.Printf("[ERR] nomad: failed to fetch tokens from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to delete tokens: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to fetch tokens from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to update tokens: %v", err)
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
