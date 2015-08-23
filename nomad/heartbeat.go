package nomad

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultHeartbeatTTL is the TTL value used for heartbeats
	// when they are first initialized. This should be longer than
	// the usual TTL since clients are switching to a new leader.
	defaultHeartbeatTTL = 300 * time.Second

	// minHeartbeatTTL is the minimum heartbeat interval.
	minHeartbeatTTL = 15 * time.Second

	// maxHeartbeatsPerSecond is the targeted maximum rate of heartbeats.
	// As the cluster size grows, we simply increase the heartbeat TTL
	// to approach this value.
	maxHeartbeatsPerSecond = 50.0
)

// initializeHeartbeatTimers is used when a leader is newly elected to create
// a new map to track heartbeat expiration and to reset all the timers from
// the previously known set of timers.
func (s *Server) initializeHeartbeatTimers() error {
	// Scan all nodes and reset their timer
	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// Get an iterator over nodes
	iter, err := snap.Nodes()
	if err != nil {
		return err
	}

	s.heartbeatTimersLock.Lock()
	defer s.heartbeatTimersLock.Unlock()

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
		s.resetHeartbeatTimerLocked(node.ID, defaultHeartbeatTTL)
	}
	return nil
}

// resetHeartbeatTimer is used to reset the TTL of a heartbeat.
// This can be used for new heartbeats and existing ones.
func (s *Server) resetHeartbeatTimer(id string) (time.Duration, error) {
	s.heartbeatTimersLock.Lock()
	defer s.heartbeatTimersLock.Unlock()

	// Compute the target TTL value
	n := len(s.heartbeatTimers)
	ttl := rateScaledInterval(maxHeartbeatsPerSecond, minHeartbeatTTL, n)

	// Reset the TTL
	s.resetHeartbeatTimerLocked(id, ttl)
	return ttl, nil
}

// resetHeartbeatTimerLocked is used to reset a heartbeat timer
// assuming the heartbeatTimerLock is already held
func (s *Server) resetHeartbeatTimerLocked(id string, ttl time.Duration) {
	// Ensure a timer map exists
	if s.heartbeatTimers == nil {
		s.heartbeatTimers = make(map[string]*time.Timer)
	}

	// Adjust the given TTL by adding an additional 10% grace period.
	// This is to compensate for network and processing delays.
	// The contract is that a heartbeat is not expired  before the TTL,
	// but there is no explicit promise about the upper bound so this is allowable.
	ttl = ttl + (ttl / 10)

	// Renew the heartbeat timer if it exists
	if timer, ok := s.heartbeatTimers[id]; ok {
		timer.Reset(ttl)
		return
	}

	// Create a new timer to track expiration of thi sheartbeat
	timer := time.AfterFunc(ttl, func() {
		s.invalidateHeartbeat(id)
	})
	s.heartbeatTimers[id] = timer
}

// invalidateHeartbeat is invoked when a heartbeat TTL is reached and we
// need to invalidate the heartbeat.
func (s *Server) invalidateHeartbeat(id string) {
	defer metrics.MeasureSince([]string{"nomad", "heartbeat", "invalidate"}, time.Now())
	// Clear the heartbeat timer
	s.heartbeatTimersLock.Lock()
	delete(s.heartbeatTimers, id)
	s.heartbeatTimersLock.Unlock()
	s.logger.Printf("[DEBUG] nomad.heartbeat: node '%s' TTL expired", id)

	// Make a request to update the node status
	req := structs.NodeUpdateStatusRequest{
		NodeID: id,
		Status: structs.NodeStatusDown,
		WriteRequest: structs.WriteRequest{
			Region: s.config.Region,
		},
	}
	var resp structs.NodeUpdateResponse
	if err := s.endpoints.Client.UpdateStatus(&req, &resp); err != nil {
		s.logger.Printf("[ERR] nomad.heartbeat: update status failed: %v", err)
	}
}

// clearHeartbeatTimer is used to clear the heartbeat time for
// a single heartbeat. This is used when a heartbeat is destroyed
// explicitly and no longer needed.
func (s *Server) clearHeartbeatTimer(id string) error {
	s.heartbeatTimersLock.Lock()
	defer s.heartbeatTimersLock.Unlock()

	if timer, ok := s.heartbeatTimers[id]; ok {
		timer.Stop()
		delete(s.heartbeatTimers, id)
	}
	return nil
}

// clearAllHeartbeatTimers is used when a leader is stepping
// down and we no longer need to track any heartbeat timers.
func (s *Server) clearAllHeartbeatTimers() error {
	s.heartbeatTimersLock.Lock()
	defer s.heartbeatTimersLock.Unlock()

	for _, t := range s.heartbeatTimers {
		t.Stop()
	}
	s.heartbeatTimers = nil
	return nil
}

// heartbeatStats is a long running routine used to capture
// the number of active heartbeats being tracked
func (s *Server) heartbeatStats() {
	for {
		select {
		case <-time.After(5 * time.Second):
			s.heartbeatTimersLock.Lock()
			num := len(s.heartbeatTimers)
			s.heartbeatTimersLock.Unlock()
			metrics.SetGauge([]string{"nomad", "heartbeat", "active"}, float32(num))

		case <-s.shutdownCh:
			return
		}
	}
}
