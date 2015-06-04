package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// monitorLeadership is used to monitor if we acquire or lose our role
// as the leader in the Raft cluster. There is some work the leader is
// expected to do, so we must react to changes
func (s *Server) monitorLeadership() {
	leaderCh := s.raft.LeaderCh()
	var stopCh chan struct{}
	for {
		select {
		case isLeader := <-leaderCh:
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
	// Wait until leadership is lost
	for {
		select {
		case <-stopCh:
			return
		case <-s.shutdownCh:
			return
		case member := <-s.reconcileCh:
			s.reconcileMember(member)
		}
	}
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

// addRaftPeer is used to add a new Raft peer when a Nomad server joins
func (s *Server) addRaftPeer(m serf.Member, parts *serverParts) error {
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

	// Attempt to add as a peer
	future := s.raft.AddPeer(parts.Addr.String())
	if err := future.Error(); err != nil && err != raft.ErrKnownPeer {
		s.logger.Printf("[ERR] nomad: failed to add raft peer: %v", err)
		return err
	}
	return nil
}

// removeRaftPeer is used to remove a Raft peer when a Nomad server leaves
// or is reaped
func (s *Server) removeRaftPeer(m serf.Member, parts *serverParts) error {
	// Attempt to remove as peer
	future := s.raft.RemovePeer(parts.Addr.String())
	if err := future.Error(); err != nil && err != raft.ErrUnknownPeer {
		s.logger.Printf("[ERR] nomad: failed to remove raft peer '%v': %v",
			parts, err)
		return err
	} else if err == nil {
		s.logger.Printf("[INFO] nomad: removed server '%s' as peer", m.Name)
	}
	return nil
}
