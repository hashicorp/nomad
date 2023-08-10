// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// heartbeatNotLeader is the error string returned when the heartbeat request
	// couldn't be completed since the server is not the leader.
	heartbeatNotLeader = "failed to reset heartbeat since server is not leader"

	// NodeHeartbeatEventMissed is the event used when the Nodes heartbeat is
	// missed.
	NodeHeartbeatEventMissed = "Node heartbeat missed"
)

var (
	// heartbeatNotLeaderErr is the error returned when the heartbeat request
	// couldn't be completed since the server is not the leader.
	heartbeatNotLeaderErr = errors.New(heartbeatNotLeader)
)

// nodeHeartbeater is used to track expiration times of node heartbeats. If it
// detects an expired node, the node status is updated to be 'down'.
type nodeHeartbeater struct {
	*Server
	logger log.Logger

	// heartbeatTimers track the expiration time of each heartbeat that has
	// a TTL. On expiration, the node status is updated to be 'down'.
	heartbeatTimers     map[string]*time.Timer
	heartbeatTimersLock sync.Mutex
}

// newNodeHeartbeater returns a new node heartbeater used to detect and act on
// failed node heartbeats.
func newNodeHeartbeater(s *Server) *nodeHeartbeater {
	return &nodeHeartbeater{
		Server: s,
		logger: s.logger.Named("heartbeat"),
	}
}

// initializeHeartbeatTimers is used when a leader is newly elected to create
// a new map to track heartbeat expiration and to reset all the timers from
// the previously known set of timers.
func (h *nodeHeartbeater) initializeHeartbeatTimers() error {
	// Scan all nodes and reset their timer
	snap, err := h.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// Get an iterator over nodes
	ws := memdb.NewWatchSet()
	iter, err := snap.Nodes(ws)
	if err != nil {
		return err
	}

	h.heartbeatTimersLock.Lock()
	defer h.heartbeatTimersLock.Unlock()

	// Handle each node
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		node := raw.(*structs.Node)
		if node.TerminalStatus() {
			continue
		}
		h.resetHeartbeatTimerLocked(node.ID, h.config.FailoverHeartbeatTTL)
	}
	return nil
}

// resetHeartbeatTimer is used to reset the TTL of a heartbeat.
// This can be used for new heartbeats and existing ones.
func (h *nodeHeartbeater) resetHeartbeatTimer(id string) (time.Duration, error) {
	h.heartbeatTimersLock.Lock()
	defer h.heartbeatTimersLock.Unlock()

	// Do not create a timer for the node since we are not the leader. This
	// check avoids the race in which leadership is lost but a timer is created
	// on this server since it was servicing an RPC during a leadership loss.
	if !h.IsLeader() {
		h.logger.Debug("ignoring resetting node TTL since this server is not the leader", "node_id", id)
		return 0, heartbeatNotLeaderErr
	}

	// Compute the target TTL value
	n := len(h.heartbeatTimers)
	ttl := helper.RateScaledInterval(h.config.MaxHeartbeatsPerSecond, h.config.MinHeartbeatTTL, n)
	ttl += helper.RandomStagger(ttl)

	// Reset the TTL
	h.resetHeartbeatTimerLocked(id, ttl+h.config.HeartbeatGrace)
	return ttl, nil
}

// resetHeartbeatTimerLocked is used to reset a heartbeat timer
// assuming the heartbeatTimerLock is already held
func (h *nodeHeartbeater) resetHeartbeatTimerLocked(id string, ttl time.Duration) {
	// Ensure a timer map exists
	if h.heartbeatTimers == nil {
		h.heartbeatTimers = make(map[string]*time.Timer)
	}

	// Renew the heartbeat timer if it exists
	if timer, ok := h.heartbeatTimers[id]; ok {
		timer.Reset(ttl)
		return
	}

	// Create a new timer to track expiration of this heartbeat
	timer := time.AfterFunc(ttl, func() {
		h.invalidateHeartbeat(id)
	})
	h.heartbeatTimers[id] = timer
}

// invalidateHeartbeat is invoked when a heartbeat TTL is reached and we
// need to invalidate the heartbeat.
func (h *nodeHeartbeater) invalidateHeartbeat(id string) {
	defer metrics.MeasureSince([]string{"nomad", "heartbeat", "invalidate"}, time.Now())
	// Clear the heartbeat timer
	h.heartbeatTimersLock.Lock()
	if timer, ok := h.heartbeatTimers[id]; ok {
		timer.Stop()
		delete(h.heartbeatTimers, id)
	}
	h.heartbeatTimersLock.Unlock()

	// Do not invalidate the node since we are not the leader. This check avoids
	// the race in which leadership is lost but a timer is created on this
	// server since it was servicing an RPC during a leadership loss.
	if !h.IsLeader() {
		h.logger.Debug("ignoring node TTL since this server is not the leader", "node_id", id)
		return
	}

	h.logger.Warn("node TTL expired", "node_id", id)

	canDisconnect, hasPendingReconnects := h.disconnectState(id)

	// Make a request to update the node status
	req := structs.NodeUpdateStatusRequest{
		NodeID:    id,
		Status:    structs.NodeStatusDown,
		NodeEvent: structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemCluster).SetMessage(NodeHeartbeatEventMissed),
		WriteRequest: structs.WriteRequest{
			Region: h.config.Region,
		},
	}

	if canDisconnect && hasPendingReconnects {
		req.Status = structs.NodeStatusDisconnected
	}
	var resp structs.NodeUpdateResponse

	if err := h.RPC("Node.UpdateStatus", &req, &resp); err != nil {
		h.logger.Error("update node status failed", "error", err)
	}
}

func (h *nodeHeartbeater) disconnectState(id string) (bool, bool) {
	node, err := h.State().NodeByID(nil, id)
	if err != nil {
		h.logger.Error("error retrieving node by id", "error", err)
		return false, false
	}

	// Exit if the node is already down or just initializing.
	if node.Status == structs.NodeStatusDown || node.Status == structs.NodeStatusInit {
		return false, false
	}

	allocs, err := h.State().AllocsByNode(nil, id)
	if err != nil {
		h.logger.Error("error retrieving allocs by node", "error", err)
		return false, false
	}

	now := time.Now().UTC()
	// Check if the node has any allocs that are configured with max_client_disconnect,
	// that are past the disconnect window, and if so, whether it has at least one
	// alloc that isn't yet expired.
	nodeCanDisconnect := false
	for _, alloc := range allocs {
		allocCanDisconnect := alloc.DisconnectTimeout(now).After(now)
		// Only process this until we find that at least one alloc is configured
		// with max_client_disconnect.
		if !nodeCanDisconnect && allocCanDisconnect {
			nodeCanDisconnect = true
		}
		// Only process this until we find one that we want to run and has not
		// yet expired.
		if allocCanDisconnect &&
			alloc.DesiredStatus == structs.AllocDesiredStatusRun &&
			!alloc.Expired(now) {
			return true, true
		}
	}

	return nodeCanDisconnect, false
}

// clearHeartbeatTimer is used to clear the heartbeat time for
// a single heartbeat. This is used when a heartbeat is destroyed
// explicitly and no longer needed.
func (h *nodeHeartbeater) clearHeartbeatTimer(id string) error {
	h.heartbeatTimersLock.Lock()
	defer h.heartbeatTimersLock.Unlock()

	if timer, ok := h.heartbeatTimers[id]; ok {
		timer.Stop()
		delete(h.heartbeatTimers, id)
	}
	return nil
}

// clearAllHeartbeatTimers is used when a leader is stepping
// down and we no longer need to track any heartbeat timers.
func (h *nodeHeartbeater) clearAllHeartbeatTimers() error {
	h.heartbeatTimersLock.Lock()
	defer h.heartbeatTimersLock.Unlock()

	for _, t := range h.heartbeatTimers {
		t.Stop()
	}
	h.heartbeatTimers = nil
	return nil
}

// heartbeatStats is a long running routine used to capture
// the number of active heartbeats being tracked
func (h *nodeHeartbeater) heartbeatStats() {
	for {
		select {
		case <-time.After(5 * time.Second):
			h.heartbeatTimersLock.Lock()
			num := len(h.heartbeatTimers)
			h.heartbeatTimersLock.Unlock()
			metrics.SetGauge([]string{"nomad", "heartbeat", "active"}, float32(num))

		case <-h.shutdownCh:
			return
		}
	}
}
