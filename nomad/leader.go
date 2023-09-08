// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"
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

var minOneTimeAuthenticationTokenVersion = version.Must(version.NewVersion("1.1.0"))

// minACLRoleVersion is the Nomad version at which the ACL role table was
// introduced. It forms the minimum version all federated servers must meet
// before the feature can be used.
var minACLRoleVersion = version.Must(version.NewVersion("1.4.0"))

// minACLAuthMethodVersion is the Nomad version at which the ACL auth methods
// table was introduced. It forms the minimum version all federated servers must
// meet before the feature can be used.
var minACLAuthMethodVersion = version.Must(version.NewVersion("1.5.0"))

// minACLJWTAuthMethodVersion is the Nomad version at which the ACL JWT auth method type
// was introduced. It forms the minimum version all federated servers must
// meet before the feature can be used.
var minACLJWTAuthMethodVersion = version.Must(version.NewVersion("1.5.4"))

// minACLBindingRuleVersion is the Nomad version at which the ACL binding rules
// table was introduced. It forms the minimum version all federated servers
// must meet before the feature can be used.
var minACLBindingRuleVersion = version.Must(version.NewVersion("1.5.0"))

// minNomadServiceRegistrationVersion is the Nomad version at which the service
// registrations table was introduced. It forms the minimum version all local
// servers must meet before the feature can be used.
var minNomadServiceRegistrationVersion = version.Must(version.NewVersion("1.3.0"))

// Any writes to node pools requires that all servers are on version 1.6.0 to
// prevent older versions of the server from crashing.
var minNodePoolsVersion = version.Must(version.NewVersion("1.6.0"))

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
			if weAreLeaderCh != nil {
				leaderStep(false)
			}
			return
		}
	}
}

func (s *Server) leadershipTransfer() error {
	retryCount := 3
	for i := 0; i < retryCount; i++ {
		err := s.raft.LeadershipTransfer().Error()
		if err == nil {
			s.logger.Info("successfully transferred leadership")
			return nil
		}

		// Don't retry if the Raft version doesn't support leadership transfer
		// since this will never succeed.
		if err == raft.ErrUnsupportedProtocol {
			return fmt.Errorf("leadership transfer not supported with Raft version lower than 3")
		}

		s.logger.Error("failed to transfer leadership attempt, will retry",
			"attempt", i,
			"retry_limit", retryCount,
			"error", err,
		)
	}
	return fmt.Errorf("failed to transfer leadership in %d attempts", retryCount)
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

			// Attempt to transfer leadership. If successful, leave the
			// leaderLoop since this node is no longer the leader. Otherwise
			// try to establish leadership again after 5 seconds.
			if err := s.leadershipTransfer(); err != nil {
				s.logger.Error("failed to transfer leadership", "error", err)
				interval = time.After(5 * time.Second)
				goto WAIT
			}
			return
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
	// Wait until leadership is lost or periodically reconcile as long as we
	// are the leader, or when Serf events arrive.
	for {
		select {
		case <-stopCh:
			// Lost leadership.
			return
		case <-s.shutdownCh:
			return
		case <-interval:
			goto RECONCILE
		case member := <-reconcileCh:
			s.reconcileMember(member)
		case errCh := <-s.reassertLeaderCh:
			// Recompute leader state, by asserting leadership and
			// repopulating leader states.

			// Check first if we are indeed the leaders first. We
			// can get into this state when the initial
			// establishLeadership has failed.
			// Afterwards we will be waiting for the interval to
			// trigger a reconciliation and can potentially end up
			// here. There is no point to reassert because this
			// agent was never leader in the first place.
			if !establishedLeader {
				errCh <- fmt.Errorf("leadership has not been established")
				continue
			}

			// refresh leadership state
			s.revokeLeadership()
			err := s.establishLeadership(stopCh)
			errCh <- err

			// In case establishLeadership fails, try to transfer leadership.
			// At this point Raft thinks we are the leader, but Nomad did not
			// complete the required steps to act as the leader.
			if err != nil {
				if err := s.leadershipTransfer(); err != nil {
					// establishedLeader was true before, but it no longer is
					// since we revoked leadership and leadershipTransfer also
					// failed.
					// Stay in the leaderLoop with establishedLeader set to
					// false so we try to establish leadership again in the
					// next loop.
					establishedLeader = false
					interval = time.After(5 * time.Second)
					goto WAIT
				}

				// leadershipTransfer was successful and it is
				// time to leave the leaderLoop.
				return
			}
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
	s.handlePausableWorkers(true)

	// Initialize and start the autopilot routine
	s.getOrCreateAutopilotConfig()
	s.autopilot.Start(s.shutdownCtx)

	// Initialize scheduler configuration.
	schedulerConfig := s.getOrCreateSchedulerConfig()

	// Initialize the ClusterID
	_, _ = s.ClusterID()
	// todo: use cluster ID for stuff, later!

	// Enable the plan queue, since we are now the leader
	s.planQueue.SetEnabled(true)

	// Start the plan evaluator
	go s.planApply()

	// Start the eval broker and blocked eval broker if these are not paused by
	// the operator.
	restoreEvals := s.handleEvalBrokerStateChange(schedulerConfig)

	// Enable the deployment watcher, since we are now the leader
	s.deploymentWatcher.SetEnabled(true, s.State())

	// Enable the NodeDrainer
	s.nodeDrainer.SetEnabled(true, s.State())

	// Enable the volume watcher, since we are now the leader
	s.volumeWatcher.SetEnabled(true, s.State(), s.getLeaderAcl())

	// Restore the eval broker state and blocked eval state. If these are
	// currently paused, we do not need to do this.
	if restoreEvals {
		if err := s.restoreEvals(); err != nil {
			return err
		}
	}

	// Activate the vault client
	s.vault.SetActive(true)

	// Enable the periodic dispatcher, since we are now the leader.
	s.periodicDispatcher.SetEnabled(true)

	// Activate RPC now that local FSM caught up with Raft (as evident by Barrier call success)
	// and all leader related components (e.g. broker queue) are enabled.
	// Auxiliary processes (e.g. background, bookkeeping, and cleanup tasks can start after)
	s.setConsistentReadReady()

	// Further clean ups and follow up that don't block RPC consistency

	// Create the first root key if it doesn't already exist
	go s.initializeKeyring(stopCh)

	// Restore the periodic dispatcher state
	if err := s.restorePeriodicDispatcher(); err != nil {
		return err
	}

	// Schedule periodic jobs which include expired local ACL token garbage
	// collection.
	go s.schedulePeriodic(stopCh)

	// Reap any failed evaluations
	go s.reapFailedEvaluations(stopCh)

	// Reap any duplicate blocked evaluations
	go s.reapDupBlockedEvaluations(stopCh)

	// Reap any cancelable evaluations
	s.reapCancelableEvalsCh = s.reapCancelableEvaluations(stopCh)

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

	// If ACLs are enabled, the leader needs to start a number of long-lived
	// routines. Exactly which routines, depends on whether this leader is
	// running within the authoritative region or not.
	if s.config.ACLEnabled {

		// The authoritative region is responsible for garbage collecting
		// expired global tokens. Otherwise, non-authoritative regions need to
		// replicate policies, tokens, and namespaces.
		switch s.config.AuthoritativeRegion {
		case s.config.Region:
			go s.schedulePeriodicAuthoritative(stopCh)
		default:
			go s.replicateACLPolicies(stopCh)
			go s.replicateACLTokens(stopCh)
			go s.replicateACLRoles(stopCh)
			go s.replicateACLAuthMethods(stopCh)
			go s.replicateACLBindingRules(stopCh)
			go s.replicateNamespaces(stopCh)
			go s.replicateNodePools(stopCh)
		}
	}

	// Setup any enterprise systems required.
	if err := s.establishEnterpriseLeadership(stopCh); err != nil {
		return err
	}

	// Cleanup orphaned Vault token accessors
	if err := s.revokeVaultAccessorsOnRestore(); err != nil {
		return err
	}

	// Cleanup orphaned Service Identity token accessors
	if err := s.revokeSITokenAccessorsOnRestore(); err != nil {
		return err
	}

	return nil
}

// replicateNamespaces is used to replicate namespaces from the authoritative
// region to this region.
func (s *Server) replicateNamespaces(stopCh chan struct{}) {
	req := structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting namespace replication from authoritative region", "region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// Rate limit how often we attempt replication
		limiter.Wait(context.Background())

		// Fetch the list of namespaces
		var resp structs.NamespaceListResponse
		req.AuthToken = s.ReplicationToken()
		err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.ListNamespaces", &req, &resp)
		if err != nil {
			s.logger.Error("failed to fetch namespaces from authoritative region", "error", err)
			goto ERR_WAIT
		}

		// Perform a two-way diff
		delete, update := diffNamespaces(s.State(), req.MinQueryIndex, resp.Namespaces)

		// Delete namespaces that should not exist
		if len(delete) > 0 {
			args := &structs.NamespaceDeleteRequest{
				Namespaces: delete,
			}
			_, _, err := s.raftApply(structs.NamespaceDeleteRequestType, args)
			if err != nil {
				s.logger.Error("failed to delete namespaces", "error", err)
				goto ERR_WAIT
			}
		}

		// Fetch any outdated namespaces
		var fetched []*structs.Namespace
		if len(update) > 0 {
			req := structs.NamespaceSetRequest{
				Namespaces: update,
				QueryOptions: structs.QueryOptions{
					Region:        s.config.AuthoritativeRegion,
					AuthToken:     s.ReplicationToken(),
					AllowStale:    true,
					MinQueryIndex: resp.Index - 1,
				},
			}
			var reply structs.NamespaceSetResponse
			if err := s.forwardRegion(s.config.AuthoritativeRegion, "Namespace.GetNamespaces", &req, &reply); err != nil {
				s.logger.Error("failed to fetch namespaces from authoritative region", "error", err)
				goto ERR_WAIT
			}
			for _, namespace := range reply.Namespaces {
				fetched = append(fetched, namespace)
			}
		}

		// Update local namespaces
		if len(fetched) > 0 {
			args := &structs.NamespaceUpsertRequest{
				Namespaces: fetched,
			}
			_, _, err := s.raftApply(structs.NamespaceUpsertRequestType, args)
			if err != nil {
				s.logger.Error("failed to update namespaces", "error", err)
				goto ERR_WAIT
			}
		}

		// Update the minimum query index, blocks until there is a change.
		req.MinQueryIndex = resp.Index
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

func (s *Server) handlePausableWorkers(isLeader bool) {
	for _, w := range s.pausableWorkers() {
		if isLeader {
			w.Pause()
		} else {
			w.Resume()
		}
	}
}

// diffNamespaces is used to perform a two-way diff between the local namespaces
// and the remote namespaces to determine which namespaces need to be deleted or
// updated.
func diffNamespaces(state *state.StateStore, minIndex uint64, remoteList []*structs.Namespace) (delete []string, update []string) {
	// Construct a set of the local and remote namespaces
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local namespaces
	iter, err := state.Namespaces(nil)
	if err != nil {
		panic("failed to iterate local namespaces")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		namespace := raw.(*structs.Namespace)
		local[namespace.Name] = namespace.Hash
	}

	// Iterate over the remote namespaces
	for _, rns := range remoteList {
		remote[rns.Name] = struct{}{}

		// Check if the namespace is missing locally
		if localHash, ok := local[rns.Name]; !ok {
			update = append(update, rns.Name)

			// Check if the namespace is newer remotely and there is a hash
			// mis-match.
		} else if rns.ModifyIndex > minIndex && !bytes.Equal(localHash, rns.Hash) {
			update = append(update, rns.Name)
		}
	}

	// Check if namespaces should be deleted
	for lns := range local {
		if _, ok := remote[lns]; !ok {
			delete = append(delete, lns)
		}
	}
	return
}

// replicateNodePools is used to replicate node pools from the authoritative
// region to this region.
func (s *Server) replicateNodePools(stopCh chan struct{}) {
	req := structs.NodePoolListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting node pool replication from authoritative region", "region", req.Region)

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// Rate limit how often we attempt replication
		limiter.Wait(context.Background())

		if !ServersMeetMinimumVersion(
			s.serf.Members(), s.Region(), minNodePoolsVersion, true) {
			s.logger.Trace(
				"all servers must be upgraded to 1.6.0 before Node Pools can be replicated")
			if s.replicationBackoffContinue(stopCh) {
				continue
			} else {
				return
			}
		}

		var resp structs.NodePoolListResponse
		req.AuthToken = s.ReplicationToken()
		err := s.forwardRegion(s.config.AuthoritativeRegion, "NodePool.List", &req, &resp)
		if err != nil {
			s.logger.Error("failed to fetch node pools from authoritative region", "error", err)
			if s.replicationBackoffContinue(stopCh) {
				continue
			} else {
				return
			}
		}

		// Perform a two-way diff
		delete, update := diffNodePools(s.State(), req.MinQueryIndex, resp.NodePools)

		// A significant amount of time could pass between the last check
		// on whether we should stop the replication process. Therefore, do
		// a check here, before calling Raft.
		select {
		case <-stopCh:
			return
		default:
		}

		// Delete node pools that should not exist
		if len(delete) > 0 {
			args := &structs.NodePoolDeleteRequest{
				Names: delete,
			}
			_, _, err := s.raftApply(structs.NodePoolDeleteRequestType, args)
			if err != nil {
				s.logger.Error("failed to delete node pools", "error", err)
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}
		}

		// Update local node pools
		if len(update) > 0 {
			args := &structs.NodePoolUpsertRequest{
				NodePools: update,
			}
			_, _, err := s.raftApply(structs.NodePoolUpsertRequestType, args)
			if err != nil {
				s.logger.Error("failed to update node pools", "error", err)
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}
		}

		// Update the minimum query index, blocks until there is a change.
		req.MinQueryIndex = resp.Index
	}
}

// diffNodePools is used to perform a two-way diff between the local node pools
// and the remote node pools to determine which node pools need to be deleted or
// updated.
func diffNodePools(store *state.StateStore, minIndex uint64, remoteList []*structs.NodePool) (delete []string, update []*structs.NodePool) {
	// Construct a set of the local and remote node pools
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local node pools
	iter, err := store.NodePools(nil, state.SortDefault)
	if err != nil {
		panic("failed to iterate local node pools")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		pool := raw.(*structs.NodePool)
		local[pool.Name] = pool.Hash
	}

	for _, rnp := range remoteList {
		remote[rnp.Name] = struct{}{}

		if localHash, ok := local[rnp.Name]; !ok {
			// Node pools that are missing locally should be added
			update = append(update, rnp)

		} else if rnp.ModifyIndex > minIndex && !bytes.Equal(localHash, rnp.Hash) {
			// Node pools that have been added/updated more recently than the
			// last index we saw, and have a hash mismatch with what we have
			// locally, should be updated.
			update = append(update, rnp)
		}
	}

	// Node pools that don't exist on the remote should be deleted
	for lnp := range local {
		if _, ok := remote[lnp]; !ok {
			delete = append(delete, lnp)
		}
	}
	return
}

// restoreEvals is used to restore pending evaluations into the eval broker and
// blocked evaluations into the blocked eval tracker. The broker and blocked
// eval tracker is maintained only by the leader, so it must be restored anytime
// a leadership transition takes place.
func (s *Server) restoreEvals() error {
	// Get an iterator over every evaluation
	ws := memdb.NewWatchSet()
	iter, err := s.fsm.State().Evals(ws, false)
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
		s.logger.Info("revoking vault accessors after becoming leader", "accessors", len(revoke))

		if err := s.vault.MarkForRevocation(revoke); err != nil {
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
		return fmt.Errorf("failed to get SI token accessors: %w", err)
	}

	var toRevoke []*structs.SITokenAccessor
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		accessor := raw.(*structs.SITokenAccessor)

		// Check the allocation
		alloc, err := fsmState.AllocByID(ws, accessor.AllocID)
		if err != nil {
			return fmt.Errorf("failed to lookup alloc %q: %w", accessor.AllocID, err)
		}
		if alloc == nil || alloc.Terminated() {
			// no longer running and associated accessors should be revoked
			toRevoke = append(toRevoke, accessor)
			continue
		}

		// Check the node
		node, err := fsmState.NodeByID(ws, accessor.NodeID)
		if err != nil {
			return fmt.Errorf("failed to lookup node %q: %w", accessor.NodeID, err)
		}
		if node == nil || node.TerminalStatus() {
			// node is terminal and associated accessors should be revoked
			toRevoke = append(toRevoke, accessor)
			continue
		}
	}

	if len(toRevoke) > 0 {
		s.logger.Info("revoking consul accessors after becoming leader", "accessors", len(toRevoke))
		s.consulACLs.MarkForRevocation(toRevoke)
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

		// We skip if the job doesn't allow overlap and there are already
		// instances running
		allowed, err := s.cronJobOverlapAllowed(job)
		if err != nil {
			return fmt.Errorf("failed to get job status: %v", err)
		}
		if !allowed {
			continue
		}

		if _, err := s.periodicDispatcher.ForceEval(job.Namespace, job.ID); err != nil {
			logger.Error("force run of periodic job failed", "job", job.NamespacedID(), "error", err)
			return fmt.Errorf("force run of periodic job %q failed: %v", job.NamespacedID(), err)
		}

		logger.Debug("periodic job force run during leadership establishment", "job", job.NamespacedID())
	}

	return nil
}

// cronJobOverlapAllowed checks if the job allows for overlap and if there are already
// instances of the job running in order to determine if a new evaluation needs to
// be created upon periodic dispatcher restore
func (s *Server) cronJobOverlapAllowed(job *structs.Job) (bool, error) {
	if job.Periodic.ProhibitOverlap {
		running, err := s.periodicDispatcher.dispatcher.RunningChildren(job)
		if err != nil {
			return false, fmt.Errorf("failed to determine if periodic job has running children %q error %q", job.NamespacedID(), err)
		}

		if running {
			return false, nil
		}
	}

	return true, nil
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
	oneTimeTokenGC := time.NewTicker(s.config.OneTimeTokenGCInterval)
	defer oneTimeTokenGC.Stop()
	rootKeyGC := time.NewTicker(s.config.RootKeyGCInterval)
	defer rootKeyGC.Stop()
	variablesRekey := time.NewTicker(s.config.VariablesRekeyInterval)
	defer variablesRekey.Stop()

	// Set up the expired ACL local token garbage collection timer.
	localTokenExpiredGC, localTokenExpiredGCStop := helper.NewSafeTimer(s.config.ACLTokenExpirationGCInterval)
	defer localTokenExpiredGCStop()

	for {

		select {
		case <-evalGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobEvalGC, index))
			}
		case <-nodeGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobNodeGC, index))
			}
		case <-jobGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobJobGC, index))
			}
		case <-deploymentGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobDeploymentGC, index))
			}
		case <-csiPluginGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobCSIPluginGC, index))
			}
		case <-csiVolumeClaimGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobCSIVolumeClaimGC, index))
			}
		case <-oneTimeTokenGC.C:
			if !ServersMeetMinimumVersion(s.Members(), s.Region(), minOneTimeAuthenticationTokenVersion, false) {
				continue
			}

			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobOneTimeTokenGC, index))
			}
		case <-localTokenExpiredGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobLocalTokenExpiredGC, index))
			}
			localTokenExpiredGC.Reset(s.config.ACLTokenExpirationGCInterval)
		case <-rootKeyGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobRootKeyRotateOrGC, index))
			}
		case <-variablesRekey.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobVariablesRekey, index))
			}
		case <-stopCh:
			return
		}
	}
}

// schedulePeriodicAuthoritative is a long-lived routine intended for use on
// the leader within the authoritative region only. It periodically queues work
// onto the _core scheduler for ACL based activities such as removing expired
// global ACL tokens.
func (s *Server) schedulePeriodicAuthoritative(stopCh chan struct{}) {

	// Set up the expired ACL global token garbage collection timer.
	globalTokenExpiredGC, globalTokenExpiredGCStop := helper.NewSafeTimer(s.config.ACLTokenExpirationGCInterval)
	defer globalTokenExpiredGCStop()

	for {
		select {
		case <-globalTokenExpiredGC.C:
			if index, ok := s.getLatestIndex(); ok {
				s.evalBroker.Enqueue(s.coreJobEval(structs.CoreJobGlobalTokenExpiredGC, index))
			}
			globalTokenExpiredGC.Reset(s.config.ACLTokenExpirationGCInterval)
		case <-stopCh:
			return
		}
	}
}

// getLatestIndex is a helper function which returns the latest index from the
// state store. The boolean return indicates whether the call has been
// successful or not.
func (s *Server) getLatestIndex() (uint64, bool) {
	snapshotIndex, err := s.fsm.State().LatestIndex()
	if err != nil {
		s.logger.Error("failed to determine state store's index", "error", err)
		return 0, false
	}
	return snapshotIndex, true
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
			s.logger.Warn("eval reached delivery limit, marking as failed",
				"eval", hclog.Fmt("%#v", updateEval))

			// Core job evals that fail or span leader elections will never
			// succeed because the follow-up doesn't have the leader ACL. We
			// rely on the leader to schedule new core jobs periodically
			// instead.
			if eval.Type != structs.JobTypeCore {

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
					s.logger.Error("failed to update failed eval and create a follow-up",
						"eval", hclog.Fmt("%#v", updateEval), "error", err)
					continue
				}
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
				s.logger.Error("failed to update duplicate evals", "evals", hclog.Fmt("%#v", cancel), "error", err)
				continue
			}
		}
	}
}

// reapCancelableEvaluations is used to reap evaluations that were marked
// cancelable by the eval broker and should be canceled. These get swept up
// whenever an eval Acks, but this ensures that we don't have a straggling batch
// when the cluster doesn't have any more work to do. Returns a wake-up channel
// that can be used to trigger a new reap without waiting for the timer
func (s *Server) reapCancelableEvaluations(stopCh chan struct{}) chan struct{} {

	wakeCh := make(chan struct{}, 1)
	go func() {

		timer, cancel := helper.NewSafeTimer(s.config.EvalReapCancelableInterval)
		defer cancel()
		for {
			select {
			case <-stopCh:
				return
			case <-wakeCh:
				cancelCancelableEvals(s)
			case <-timer.C:
				cancelCancelableEvals(s)
				timer.Reset(s.config.EvalReapCancelableInterval)
			}
		}
	}()

	return wakeCh
}

const cancelableEvalsBatchSize = 728 // structs.MaxUUIDsPerWriteRequest / 10

// cancelCancelableEvals pulls a batch of cancelable evaluations from the eval
// broker and updates their status to canceled.
func cancelCancelableEvals(srv *Server) error {

	const cancelDesc = "canceled after more recent eval was processed"

	// We *can* send larger raft logs but rough benchmarks show that a smaller
	// page size strikes a balance between throughput and time we block the FSM
	// apply for other operations
	cancelable := srv.evalBroker.Cancelable(cancelableEvalsBatchSize)
	if len(cancelable) > 0 {
		for i, eval := range cancelable {
			eval = eval.Copy()
			eval.Status = structs.EvalStatusCancelled
			eval.StatusDescription = cancelDesc
			eval.UpdateModifyTime()
			cancelable[i] = eval
		}

		update := &structs.EvalUpdateRequest{
			Evals:        cancelable,
			WriteRequest: structs.WriteRequest{Region: srv.Region()},
		}
		_, _, err := srv.raftApply(structs.EvalUpdateRequestType, update)
		if err != nil {
			srv.logger.Warn("eval cancel failed", "error", err, "method", "ack")
			return err
		}
	}
	return nil
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
		metrics.SetGaugeWithLabels([]string{"nomad", "job_summary", "unknown"},
			float32(tgSummary.Unknown), labels)
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

	// Disable the eval broker and blocked evals. We do not need to check the
	// scheduler configuration paused eval broker value, as the brokers should
	// always be paused on the non-leader.
	s.brokerLock.Lock()
	s.evalBroker.SetEnabled(false)
	s.blockedEvals.SetEnabled(false)
	s.brokerLock.Unlock()

	// Disable the periodic dispatcher, since it is only useful as a leader
	s.periodicDispatcher.SetEnabled(false)

	// Disable the Vault client as it is only useful as a leader.
	s.vault.SetActive(false)

	// Disable the deployment watcher as it is only useful as a leader.
	s.deploymentWatcher.SetEnabled(false, nil)

	// Disable the node drainer
	s.nodeDrainer.SetEnabled(false, nil)

	// Disable the volume watcher
	s.volumeWatcher.SetEnabled(false, nil, "")

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
	s.handlePausableWorkers(false)

	return nil
}

// pausableWorkers returns a slice of the workers
// to pause on leader transitions.
//
// Upon leadership establishment, pause workers to free half
// the cores for use in the plan queue and evaluation broker
func (s *Server) pausableWorkers() []*Worker {
	n := len(s.workers)
	if n <= 1 {
		return []*Worker{}
	}

	// Disabling 3/4 of the workers frees CPU for raft and the
	// plan applier which uses 1/2 the cores.
	return s.workers[:3*n/4]
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
	minRaftProtocol, err := s.MinRaftProtocol()
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

	minRaftProtocol, err := s.MinRaftProtocol()
	if err != nil {
		return err
	}

	// Pick which remove API to use based on how the server was added.
	for _, server := range configFuture.Configuration().Servers {
		// Check if this is the server to remove based on how it was registered.
		// Raft v2 servers are registered by address.
		// Raft v3 servers are registered by ID.
		if server.ID == raft.ServerID(parts.ID) || server.Address == raft.ServerAddress(addr) {
			// Use the new add/remove APIs if we understand them.
			if minRaftProtocol >= 2 {
				s.logger.Info("removing server by ID", "id", server.ID)
				future := s.raft.RemoveServer(server.ID, 0, 0)
				if err := future.Error(); err != nil {
					s.logger.Error("failed to remove raft peer", "id", server.ID, "error", err)
					return err
				}
			} else {
				// If not, use the old remove API
				s.logger.Info("removing server by address", "address", server.Address)
				future := s.raft.RemovePeer(raft.ServerAddress(addr))
				if err := future.Error(); err != nil {
					s.logger.Error("failed to remove raft peer", "address", addr, "error", err)
					return err
				}
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
func diffACLTokens(store *state.StateStore, minIndex uint64, remoteList []*structs.ACLTokenListStub) (delete []string, update []string) {
	// Construct a set of the local and remote policies
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local global tokens
	iter, err := store.ACLTokensByGlobal(nil, true, state.SortDefault)
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

// replicateACLRoles is used to replicate ACL Roles from the authoritative
// region to this region. The loop should only be run on the leader within the
// federated region.
func (s *Server) replicateACLRoles(stopCh chan struct{}) {

	// Generate our request object. We only need to do this once and reuse it
	// for every RPC request. The MinQueryIndex is updated after every
	// successful replication loop, so the next query acts as a blocking query
	// and only returns upon a change in the authoritative region.
	req := structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Region:     s.config.AuthoritativeRegion,
		},
	}

	// Create our replication rate limiter for ACL roles and log a lovely
	// message to indicate the process is starting.
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting ACL Role replication from authoritative region",
		"authoritative_region", req.Region)

	// Enter the main ACL Role replication loop that will only exit when the
	// stopCh is closed.
	//
	// Any error encountered will use the replicationBackoffContinue function
	// which handles replication backoff and shutdown coordination in the event
	// of an error inside the loop.
	for {
		select {
		case <-stopCh:
			return
		default:

			// Rate limit how often we attempt replication. It is OK to ignore
			// the error as the context will never be cancelled and the limit
			// parameters are controlled internally.
			_ = limiter.Wait(context.Background())

			if !ServersMeetMinimumVersion(
				s.serf.Members(), s.Region(), minACLRoleVersion, true) {
				s.logger.Trace(
					"all servers must be upgraded to 1.4.0 or later before ACL Roles can be replicated")
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Set the replication token on each replication iteration so that
			// it is always current and can handle agent SIGHUP reloads.
			req.AuthToken = s.ReplicationToken()

			var resp structs.ACLRolesListResponse

			// Make the list RPC request to the authoritative region, so we
			// capture the latest ACL role listing.
			err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLListRolesRPCMethod, &req, &resp)
			if err != nil {
				s.logger.Error("failed to fetch ACL Roles from authoritative region", "error", err)
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Perform a two-way diff on the ACL roles.
			toDelete, toUpdate := diffACLRoles(s.State(), req.MinQueryIndex, resp.ACLRoles)

			// A significant amount of time could pass between the last check
			// on whether we should stop the replication process. Therefore, do
			// a check here, before calling Raft.
			select {
			case <-stopCh:
				return
			default:
			}

			// If we have ACL roles to delete, make this call directly to Raft.
			if len(toDelete) > 0 {
				args := structs.ACLRolesDeleteByIDRequest{ACLRoleIDs: toDelete}
				_, _, err := s.raftApply(structs.ACLRolesDeleteByIDRequestType, &args)

				// If the error was because we lost leadership while calling
				// Raft, avoid logging as this can be confusing to operators.
				if err != nil {
					if err != raft.ErrLeadershipLost {
						s.logger.Error("failed to delete ACL roles", "error", err)
					}
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Fetch any outdated policies.
			var fetched []*structs.ACLRole
			if len(toUpdate) > 0 {
				req := structs.ACLRolesByIDRequest{
					ACLRoleIDs: toUpdate,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						AuthToken:     s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.ACLRolesByIDResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLGetRolesByIDRPCMethod, &req, &reply); err != nil {
					s.logger.Error("failed to fetch ACL Roles from authoritative region", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
				for _, aclRole := range reply.ACLRoles {
					fetched = append(fetched, aclRole)
				}
			}

			// Update local tokens
			if len(fetched) > 0 {

				// The replication of ACL roles and policies are independent,
				// therefore we cannot ensure the policies linked within the
				// role are present. We must set allow missing to true.
				args := structs.ACLRolesUpsertRequest{
					ACLRoles:             fetched,
					AllowMissingPolicies: true,
				}

				// Perform the upsert directly via Raft.
				_, _, err := s.raftApply(structs.ACLRolesUpsertRequestType, &args)
				if err != nil {
					s.logger.Error("failed to update ACL roles", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Update the minimum query index, blocks until there is a change.
			req.MinQueryIndex = resp.Index
		}
	}
}

// diffACLRoles is used to perform a two-way diff between the local ACL Roles
// and the remote Roles to determine which tokens need to be deleted or
// updated. The returned array's contain ACL Role IDs.
func diffACLRoles(
	store *state.StateStore, minIndex uint64, remoteList []*structs.ACLRoleListStub) (
	delete []string, update []string) {

	// The local ACL role tracking is keyed by the role ID and the value is the
	// hash of the role.
	local := make(map[string][]byte)

	// The remote ACL role tracking is keyed by the role ID; the value is an
	// empty struct as we already have the full object.
	remote := make(map[string]struct{})

	// Read all the ACL role currently held within our local state. This panic
	// will only happen as a developer making a mistake with naming the index
	// to use.
	iter, err := store.GetACLRoles(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to iterate local ACL roles: %v", err))
	}

	// Iterate the local ACL roles and add them to our tracking of local roles.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclRole := raw.(*structs.ACLRole)
		local[aclRole.ID] = aclRole.Hash
	}

	// Iterate over the remote ACL roles.
	for _, remoteACLRole := range remoteList {
		remote[remoteACLRole.ID] = struct{}{}

		// Identify whether the ACL role is within the local state. If it is
		// not, add this to our update list.
		if localHash, ok := local[remoteACLRole.ID]; !ok {
			update = append(update, remoteACLRole.ID)

			// Check if ACL role is newer remotely and there is a hash
			// mismatch.
		} else if remoteACLRole.ModifyIndex > minIndex && !bytes.Equal(localHash, remoteACLRole.Hash) {
			update = append(update, remoteACLRole.ID)
		}
	}

	// If we have ACL roles within state which are no longer present in the
	// authoritative region we should delete them.
	for localACLRole := range local {
		if _, ok := remote[localACLRole]; !ok {
			delete = append(delete, localACLRole)
		}
	}
	return
}

// replicateACLAuthMethods is used to replicate ACL Authentication Methods from
// the authoritative region to this region. The loop should only be run on the
// leader within the federated region.
func (s *Server) replicateACLAuthMethods(stopCh chan struct{}) {

	// Generate our request object. We only need to do this once and reuse it
	// for every RPC request. The MinQueryIndex is updated after every
	// successful replication loop, so the next query acts as a blocking query
	// and only returns upon a change in the authoritative region.
	req := structs.ACLAuthMethodListRequest{
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Region:     s.config.AuthoritativeRegion,
		},
	}

	// Create our replication rate limiter for ACL auth-methods and log a
	// lovely message to indicate the process is starting.
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting ACL Auth-Methods replication from authoritative region",
		"authoritative_region", req.Region)

	// Enter the main ACL auth-methods replication loop that will only exit
	// when the stopCh is closed.
	//
	// Any error encountered will use the replicationBackoffContinue function
	// which handles replication backoff and shutdown coordination in the event
	// of an error inside the loop.
	for {
		select {
		case <-stopCh:
			return
		default:

			// Rate limit how often we attempt replication. It is OK to ignore
			// the error as the context will never be cancelled and the limit
			// parameters are controlled internally.
			_ = limiter.Wait(context.Background())

			if !ServersMeetMinimumVersion(
				s.serf.Members(), s.Region(), minACLAuthMethodVersion, true) {
				s.logger.Trace(
					"all servers must be upgraded to 1.5.0 or later before ACL Auth Methods can be replicated")
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Set the replication token on each replication iteration so that
			// it is always current and can handle agent SIGHUP reloads.
			req.AuthToken = s.ReplicationToken()

			var resp structs.ACLAuthMethodListResponse

			// Make the list RPC request to the authoritative region, so we
			// capture the latest ACL auth-method listing.
			err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLListAuthMethodsRPCMethod, &req, &resp)
			if err != nil {
				s.logger.Error("failed to fetch ACL auth-methods from authoritative region", "error", err)
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Perform a two-way diff on the ACL auth-methods.
			toDelete, toUpdate := diffACLAuthMethods(s.State(), req.MinQueryIndex, resp.AuthMethods)

			// A significant amount of time could pass between the last check
			// on whether we should stop the replication process. Therefore, do
			// a check here, before calling Raft.
			select {
			case <-stopCh:
				return
			default:
			}

			// If we have ACL auth-methods to delete, make this call directly
			// to Raft.
			if len(toDelete) > 0 {
				args := structs.ACLAuthMethodDeleteRequest{Names: toDelete}
				_, _, err := s.raftApply(structs.ACLAuthMethodsDeleteRequestType, &args)

				// If the error was because we lost leadership while calling
				// Raft, avoid logging as this can be confusing to operators.
				if err != nil {
					if err != raft.ErrLeadershipLost {
						s.logger.Error("failed to delete ACL auth-methods", "error", err)
					}
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Fetch any outdated auth-methods.
			var fetched []*structs.ACLAuthMethod
			if len(toUpdate) > 0 {
				req := structs.ACLAuthMethodsGetRequest{
					Names: toUpdate,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						AuthToken:     s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.ACLAuthMethodsGetResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLGetAuthMethodsRPCMethod, &req, &reply); err != nil {
					s.logger.Error("failed to fetch ACL auth-methods from authoritative region", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
				for _, aclAuthMethod := range reply.AuthMethods {
					fetched = append(fetched, aclAuthMethod)
				}
			}

			// Update local auth-methods.
			if len(fetched) > 0 {
				args := structs.ACLAuthMethodUpsertRequest{
					AuthMethods: fetched,
				}

				// Perform the upsert directly via Raft.
				_, _, err := s.raftApply(structs.ACLAuthMethodsUpsertRequestType, &args)
				if err != nil {
					s.logger.Error("failed to update ACL auth-methods", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Update the minimum query index, blocks until there is a change.
			req.MinQueryIndex = resp.Index
		}
	}
}

// diffACLAuthMethods is used to perform a two-way diff between the local ACL
// auth-methods and the remote auth-methods to determine which ones need to be
// deleted or updated. The returned array's contain ACL auth-method names.
func diffACLAuthMethods(
	store *state.StateStore, minIndex uint64, remoteList []*structs.ACLAuthMethodStub) (
	delete []string, update []string) {

	// The local ACL auth-method tracking is keyed by the name and the value is
	// the hash of the auth-method.
	local := make(map[string][]byte)

	// The remote ACL auth-method tracking is keyed by the name; the value is
	// an empty struct as we already have the full object.
	remote := make(map[string]struct{})

	// Read all the ACL auth-methods currently held within our local state.
	// This panic will only happen as a developer making a mistake with naming
	// the index to use.
	iter, err := store.GetACLAuthMethods(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to iterate local ACL roles: %v", err))
	}

	// Iterate the local ACL auth-methods and add them to our tracking of
	// local auth-methods
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclAuthMethod := raw.(*structs.ACLAuthMethod)
		local[aclAuthMethod.Name] = aclAuthMethod.Hash
	}

	// Iterate over the remote ACL auth-methods.
	for _, remoteACLAuthMethod := range remoteList {
		remote[remoteACLAuthMethod.Name] = struct{}{}

		// Identify whether the ACL auth-method is within the local state. If
		// it is not, add this to our update list.
		if localHash, ok := local[remoteACLAuthMethod.Name]; !ok {
			update = append(update, remoteACLAuthMethod.Name)

			// Check if ACL auth-method is newer remotely and there is a hash
			// mismatch.
		} else if remoteACLAuthMethod.ModifyIndex > minIndex && !bytes.Equal(localHash, remoteACLAuthMethod.Hash) {
			update = append(update, remoteACLAuthMethod.Name)
		}
	}

	// If we have ACL auth-methods within state which are no longer present in
	// the authoritative region we should delete them.
	for localACLAuthMethod := range local {
		if _, ok := remote[localACLAuthMethod]; !ok {
			delete = append(delete, localACLAuthMethod)
		}
	}
	return
}

// replicateACLBindingRules is used to replicate ACL binding rules from the
// authoritative region to this region. The loop should only be run on the
// leader within the federated region.
func (s *Server) replicateACLBindingRules(stopCh chan struct{}) {

	// Generate our request object. We only need to do this once and reuse it
	// for every RPC request. The MinQueryIndex is updated after every
	// successful replication loop, so the next query acts as a blocking query
	// and only returns upon a change in the authoritative region.
	req := structs.ACLBindingRulesListRequest{
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Region:     s.config.AuthoritativeRegion,
		},
	}

	// Create our replication rate limiter for ACL binding rules and log a
	// lovely message to indicate the process is starting.
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting ACL Binding Rules replication from authoritative region",
		"authoritative_region", req.Region)

	// Enter the main ACL binding rules replication loop that will only exit
	// when the stopCh is closed.
	//
	// Any error encountered will use the replicationBackoffContinue function
	// which handles replication backoff and shutdown coordination in the event
	// of an error inside the loop.
	for {
		select {
		case <-stopCh:
			return
		default:

			// Rate limit how often we attempt replication. It is OK to ignore
			// the error as the context will never be cancelled and the limit
			// parameters are controlled internally.
			_ = limiter.Wait(context.Background())

			if !ServersMeetMinimumVersion(
				s.serf.Members(), s.Region(), minACLBindingRuleVersion, true) {
				s.logger.Trace(
					"all servers must be upgraded to 1.5.0 or later before ACL Binding Rules can be replicated")
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Set the replication token on each replication iteration so that
			// it is always current and can handle agent SIGHUP reloads.
			req.AuthToken = s.ReplicationToken()

			var resp structs.ACLBindingRulesListResponse

			// Make the list RPC request to the authoritative region, so we
			// capture the latest ACL binding rules listing.
			err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLListBindingRulesRPCMethod, &req, &resp)
			if err != nil {
				s.logger.Error("failed to fetch ACL binding rules from authoritative region", "error", err)
				if s.replicationBackoffContinue(stopCh) {
					continue
				} else {
					return
				}
			}

			// Perform a two-way diff on the ACL binding rules.
			toDelete, toUpdate := diffACLBindingRules(s.State(), req.MinQueryIndex, resp.ACLBindingRules)

			// A significant amount of time could pass between the last check
			// on whether we should stop the replication process. Therefore, do
			// a check here, before calling Raft.
			select {
			case <-stopCh:
				return
			default:
			}

			// If we have ACL binding rules to delete, make this call directly
			// to Raft.
			if len(toDelete) > 0 {
				args := structs.ACLBindingRulesDeleteRequest{ACLBindingRuleIDs: toDelete}
				_, _, err := s.raftApply(structs.ACLBindingRulesDeleteRequestType, &args)

				// If the error was because we lost leadership while calling
				// Raft, avoid logging as this can be confusing to operators.
				if err != nil {
					if err != raft.ErrLeadershipLost {
						s.logger.Error("failed to delete ACL binding rules", "error", err)
					}
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Fetch any outdated binding rules.
			var fetched []*structs.ACLBindingRule
			if len(toUpdate) > 0 {
				req := structs.ACLBindingRulesRequest{
					ACLBindingRuleIDs: toUpdate,
					QueryOptions: structs.QueryOptions{
						Region:        s.config.AuthoritativeRegion,
						AuthToken:     s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.ACLBindingRulesResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion, structs.ACLGetBindingRulesRPCMethod, &req, &reply); err != nil {
					s.logger.Error("failed to fetch ACL binding rules from authoritative region", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
				for _, aclBindingRule := range reply.ACLBindingRules {
					fetched = append(fetched, aclBindingRule)
				}
			}

			// Update local binding rules.
			if len(fetched) > 0 {
				args := structs.ACLBindingRulesUpsertRequest{
					ACLBindingRules:         fetched,
					AllowMissingAuthMethods: true,
				}

				// Perform the upsert directly via Raft.
				_, _, err := s.raftApply(structs.ACLBindingRulesUpsertRequestType, &args)
				if err != nil {
					s.logger.Error("failed to update ACL binding rules", "error", err)
					if s.replicationBackoffContinue(stopCh) {
						continue
					} else {
						return
					}
				}
			}

			// Update the minimum query index, blocks until there is a change.
			req.MinQueryIndex = resp.Index
		}
	}
}

// diffACLBindingRules is used to perform a two-way diff between the local ACL
// binding rules and the remote binding rules to determine which ones need to be
// deleted or updated. The returned array's contain ACL binding rule names.
func diffACLBindingRules(
	store *state.StateStore, minIndex uint64, remoteList []*structs.ACLBindingRuleListStub) (
	delete []string, update []string) {

	// The local ACL binding rules tracking is keyed by the name and the value
	// is the hash of the auth-method.
	local := make(map[string][]byte)

	// The remote ACL binding rules tracking is keyed by the name; the value is
	// an empty struct as we already have the full object.
	remote := make(map[string]struct{})

	// Read all the ACL binding rules currently held within our local state.
	// This panic will only happen as a developer making a mistake with naming
	// the index to use.
	iter, err := store.GetACLBindingRules(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to iterate local ACL binding rules: %v", err))
	}

	// Iterate the local ACL binding rules and add them to our tracking of
	// local binding rules.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclBindingRule := raw.(*structs.ACLBindingRule)
		local[aclBindingRule.ID] = aclBindingRule.Hash
	}

	// Iterate over the remote ACL binding rules.
	for _, remoteACLBindingRule := range remoteList {
		remote[remoteACLBindingRule.ID] = struct{}{}

		// Identify whether the ACL auth-method is within the local state. If
		// it is not, add this to our update list.
		if localHash, ok := local[remoteACLBindingRule.ID]; !ok {
			update = append(update, remoteACLBindingRule.ID)

			// Check if the ACL binding rule is newer remotely and there is a
			// hash mismatch.
		} else if remoteACLBindingRule.ModifyIndex > minIndex && !bytes.Equal(localHash, remoteACLBindingRule.Hash) {
			update = append(update, remoteACLBindingRule.ID)
		}
	}

	// If we have ACL binding rules within state which are no longer present in
	// the authoritative region we should delete them.
	for localACLBindingRules := range local {
		if _, ok := remote[localACLBindingRules]; !ok {
			delete = append(delete, localACLBindingRules)
		}
	}
	return
}

// replicationBackoffContinue should be used when a replication loop encounters
// an error and wants to wait until either the backoff time has been met, or
// the stopCh has been closed. The boolean indicates whether the replication
// process should continue.
//
// Typical use:
//
//	  if s.replicationBackoffContinue(stopCh) {
//		   continue
//		 } else {
//	    return
//	  }
func (s *Server) replicationBackoffContinue(stopCh chan struct{}) bool {

	timer, timerStopFn := helper.NewSafeTimer(s.config.ReplicationBackoff)
	defer timerStopFn()

	select {
	case <-timer.C:
		return true
	case <-stopCh:
		return false
	}
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

	if !ServersMeetMinimumVersion(s.Members(), AllRegions, minAutopilotVersion, false) {
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
	if !ServersMeetMinimumVersion(s.Members(), s.Region(), minSchedulerConfigVersion, false) {
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

var minVersionKeyring = version.Must(version.NewVersion("1.4.0"))

// initializeKeyring creates the first root key if the leader doesn't
// already have one. The metadata will be replicated via raft and then
// the followers will get the key material from their own key
// replication.
func (s *Server) initializeKeyring(stopCh <-chan struct{}) {

	logger := s.logger.Named("keyring")

	store := s.fsm.State()
	keyMeta, err := store.GetActiveRootKeyMeta(nil)
	if err != nil {
		logger.Error("failed to get active key: %v", err)
		return
	}
	if keyMeta != nil {
		return
	}

	logger.Trace("verifying cluster is ready to initialize keyring")
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		if ServersMeetMinimumVersion(s.serf.Members(), s.Region(), minVersionKeyring, true) {
			break
		}
	}
	// we might have lost leadership during the version check
	if !s.IsLeader() {
		return
	}

	logger.Trace("initializing keyring")

	rootKey, err := structs.NewRootKey(structs.EncryptionAlgorithmAES256GCM)
	rootKey.Meta.SetActive()
	if err != nil {
		logger.Error("could not initialize keyring: %v", err)
		return
	}

	err = s.encrypter.AddKey(rootKey)
	if err != nil {
		logger.Error("could not add initial key to keyring: %v", err)
		return
	}

	if _, _, err = s.raftApply(structs.RootKeyMetaUpsertRequestType,
		structs.KeyringUpdateRootKeyMetaRequest{
			RootKeyMeta: rootKey.Meta,
		}); err != nil {
		logger.Error("could not initialize keyring: %v", err)
		return
	}

	logger.Info("initialized keyring", "id", rootKey.Meta.KeyID)
}

func (s *Server) generateClusterID() (string, error) {
	if !ServersMeetMinimumVersion(s.Members(), AllRegions, minClusterIDVersion, false) {
		s.logger.Named("core").Warn("cannot initialize cluster ID until all servers are above minimum version", "min_version", minClusterIDVersion)
		return "", fmt.Errorf("cluster ID cannot be created until all servers are above minimum version %s", minClusterIDVersion)
	}

	newMeta := structs.ClusterMetadata{ClusterID: uuid.Generate(), CreateTime: time.Now().UnixNano()}
	if _, _, err := s.raftApply(structs.ClusterMetadataRequestType, newMeta); err != nil {
		s.logger.Named("core").Error("failed to create cluster ID", "error", err)
		return "", fmt.Errorf("failed to create cluster ID: %w", err)
	}

	s.logger.Named("core").Info("established cluster id", "cluster_id", newMeta.ClusterID, "create_time", newMeta.CreateTime)
	return newMeta.ClusterID, nil
}

// handleEvalBrokerStateChange handles changing the evalBroker and blockedEvals
// enabled status based on the passed scheduler configuration. The boolean
// response indicates whether the caller needs to call restoreEvals() due to
// the brokers being enabled. It is for use when the change must take the
// scheduler configuration into account. This is not needed when calling
// revokeLeadership, as the configuration doesn't matter, and we need to ensure
// the brokers are stopped.
//
// The function checks the server is the leader and uses a mutex to avoid any
// potential timings problems. Consider the following timings:
//   - operator updates the configuration via the API
//   - the RPC handler applies the change via Raft
//   - leadership transitions with write barrier
//   - the RPC handler call this function to enact the change
//
// The mutex also protects against a situation where leadership is revoked
// while this function is being called. Ensuring the correct series of actions
// occurs so that state stays consistent.
func (s *Server) handleEvalBrokerStateChange(schedConfig *structs.SchedulerConfiguration) bool {

	// Grab the lock first. Once we have this we can be sure to run everything
	// needed before any leader transition can attempt to modify the state.
	s.brokerLock.Lock()
	defer s.brokerLock.Unlock()

	// If we are no longer the leader, exit early.
	if !s.IsLeader() {
		return false
	}

	// enableEvalBroker tracks whether the evalBroker and blockedEvals
	// processes should be enabled or not. It allows us to answer this question
	// whether using a persisted Raft configuration, or the default bootstrap
	// config.
	var enableBrokers, restoreEvals bool

	// The scheduler config can only be persisted to Raft once quorum has been
	// established. If this is a fresh cluster, we need to use the default
	// scheduler config, otherwise we can use the persisted object.
	switch schedConfig {
	case nil:
		enableBrokers = !s.config.DefaultSchedulerConfig.PauseEvalBroker
	default:
		enableBrokers = !schedConfig.PauseEvalBroker
	}

	// If the evalBroker status is changing, set the new state.
	if enableBrokers != s.evalBroker.Enabled() {
		s.logger.Info("eval broker status modified", "paused", !enableBrokers)
		s.evalBroker.SetEnabled(enableBrokers)
		restoreEvals = enableBrokers
	}

	// If the blockedEvals status is changing, set the new state.
	if enableBrokers != s.blockedEvals.Enabled() {
		s.logger.Info("blocked evals status modified", "paused", !enableBrokers)
		s.blockedEvals.SetEnabled(enableBrokers)
		restoreEvals = enableBrokers

		if enableBrokers {
			s.blockedEvals.SetTimetable(s.fsm.TimeTable())
		}
	}

	return restoreEvals
}
