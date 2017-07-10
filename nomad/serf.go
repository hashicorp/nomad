package nomad

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

const (
	// StatusReap is used to update the status of a node if we
	// are handling a EventMemberReap
	StatusReap = serf.MemberStatus(-1)

	// maxPeerRetries limits how many invalidate attempts are made
	maxPeerRetries = 6

	// peerRetryBase is a baseline retry time
	peerRetryBase = 1 * time.Second
)

// serfEventHandler is used to handle events from the serf cluster
func (s *Server) serfEventHandler() {
	for {
		select {
		case e := <-s.eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.nodeJoin(e.(serf.MemberEvent))
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventMemberLeave, serf.EventMemberFailed:
				s.nodeFailed(e.(serf.MemberEvent))
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventMemberReap:
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventMemberUpdate, serf.EventUser, serf.EventQuery: // Ignore
			default:
				s.logger.Printf("[WARN] nomad: unhandled serf event: %#v", e)
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// nodeJoin is used to handle join events on the serf cluster
func (s *Server) nodeJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isNomadServer(m)
		if !ok {
			s.logger.Printf("[WARN] nomad: non-server in gossip pool: %s", m.Name)
			continue
		}
		s.logger.Printf("[INFO] nomad: adding server %s", parts)

		// Check if this server is known
		found := false
		s.peerLock.Lock()
		existing := s.peers[parts.Region]
		for idx, e := range existing {
			if e.Name == parts.Name {
				existing[idx] = parts
				found = true
				break
			}
		}

		// Add ot the list if not known
		if !found {
			s.peers[parts.Region] = append(existing, parts)
		}

		// Check if a local peer
		if parts.Region == s.config.Region {
			s.localPeers[raft.ServerAddress(parts.Addr.String())] = parts
		}
		s.peerLock.Unlock()

		// If we still expecting to bootstrap, may need to handle this
		if atomic.LoadInt32(&s.config.BootstrapExpect) != 0 {
			s.maybeBootstrap()
		}
	}
}

// maybeBootstrap is used to handle bootstrapping when a new server joins
func (s *Server) maybeBootstrap() {
	// Bootstrap can only be done if there are no committed logs, remove our
	// expectations of bootstrapping. This is slightly cheaper than the full
	// check that BootstrapCluster will do, so this is a good pre-filter.
	var index uint64
	var err error
	if s.raftStore != nil {
		index, err = s.raftStore.LastIndex()
	} else if s.raftInmem != nil {
		index, err = s.raftInmem.LastIndex()
	} else {
		panic("neither raftInmem or raftStore is initialized")
	}
	if err != nil {
		s.logger.Printf("[ERR] nomad: failed to read last raft index: %v", err)
		return
	}

	// Bootstrap can only be done if there are no committed logs,
	// remove our expectations of bootstrapping
	if index != 0 {
		atomic.StoreInt32(&s.config.BootstrapExpect, 0)
		return
	}

	// Scan for all the known servers
	members := s.serf.Members()
	var servers []serverParts
	for _, member := range members {
		valid, p := isNomadServer(member)
		if !valid {
			continue
		}
		if p.Region != s.config.Region {
			continue
		}
		if p.Expect != 0 && p.Expect != int(atomic.LoadInt32(&s.config.BootstrapExpect)) {
			s.logger.Printf("[ERR] nomad: peer %v has a conflicting expect value. All nodes should expect the same number.", member)
			return
		}
		if p.Bootstrap {
			s.logger.Printf("[ERR] nomad: peer %v has bootstrap mode. Expect disabled.", member)
			return
		}
		servers = append(servers, *p)
	}

	// Skip if we haven't met the minimum expect count
	if len(servers) < int(atomic.LoadInt32(&s.config.BootstrapExpect)) {
		return
	}

	// Query each of the servers and make sure they report no Raft peers.
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
		},
	}
	for _, server := range servers {
		var peers []string

		// Retry with exponential backoff to get peer status from this server
		for attempt := uint(0); attempt < maxPeerRetries; attempt++ {
			if err := s.connPool.RPC(s.config.Region, server.Addr, server.MajorVersion,
				"Status.Peers", req, &peers); err != nil {
				nextRetry := time.Duration((1 << attempt) * peerRetryBase)
				s.logger.Printf("[ERR] consul: Failed to confirm peer status for %s: %v. Retrying in "+
					"%v...", server.Name, err, nextRetry.String())
				time.Sleep(nextRetry)
			} else {
				break
			}
		}

		// Found a node with some Raft peers, stop bootstrap since there's
		// evidence of an existing cluster. We should get folded in by the
		// existing servers if that's the case, so it's cleaner to sit as a
		// candidate with no peers so we don't cause spurious elections.
		// It's OK this is racy, because even with an initial bootstrap
		// as long as one peer runs bootstrap things will work, and if we
		// have multiple peers bootstrap in the same way, that's OK. We
		// just don't want a server added much later to do a live bootstrap
		// and interfere with the cluster. This isn't required for Raft's
		// correctness because no server in the existing cluster will vote
		// for this server, but it makes things much more stable.
		if len(peers) > 0 {
			s.logger.Printf("[INFO] nomad: Existing Raft peers reported by %s (%v), disabling bootstrap mode", server.Name, server.Addr)
			atomic.StoreInt32(&s.config.BootstrapExpect, 0)
			return
		}
	}

	// Update the peer set
	// Attempt a live bootstrap!
	var configuration raft.Configuration
	var addrs []string
	for _, server := range servers {
		addr := server.Addr.String()
		addrs = append(addrs, addr)
		peer := raft.Server{
			ID:      raft.ServerID(addr),
			Address: raft.ServerAddress(addr),
		}
		configuration.Servers = append(configuration.Servers, peer)
	}
	s.logger.Printf("[INFO] nomad: Found expected number of peers (%s), attempting to bootstrap cluster...",
		strings.Join(addrs, ","))
	future := s.raft.BootstrapCluster(configuration)
	if err := future.Error(); err != nil {
		s.logger.Printf("[ERR] nomad: Failed to bootstrap cluster: %v", err)
	}

	// Bootstrapping complete, or failed for some reason, don't enter this again
	atomic.StoreInt32(&s.config.BootstrapExpect, 0)
}

// nodeFailed is used to handle fail events on the serf cluster
func (s *Server) nodeFailed(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, parts := isNomadServer(m)
		if !ok {
			continue
		}
		s.logger.Printf("[INFO] nomad: removing server %s", parts)

		// Remove the server if known
		s.peerLock.Lock()
		existing := s.peers[parts.Region]
		n := len(existing)
		for i := 0; i < n; i++ {
			if existing[i].Name == parts.Name {
				existing[i], existing[n-1] = existing[n-1], nil
				existing = existing[:n-1]
				n--
				break
			}
		}

		// Trim the list there are no known servers in a region
		if n == 0 {
			delete(s.peers, parts.Region)
		} else {
			s.peers[parts.Region] = existing
		}

		// Check if local peer
		if parts.Region == s.config.Region {
			delete(s.localPeers, raft.ServerAddress(parts.Addr.String()))
		}
		s.peerLock.Unlock()
	}
}

// localMemberEvent is used to reconcile Serf events with the
// consistent store if we are the current leader.
func (s *Server) localMemberEvent(me serf.MemberEvent) {
	// Do nothing if we are not the leader
	if !s.IsLeader() {
		return
	}

	// Check if this is a reap event
	isReap := me.EventType() == serf.EventMemberReap

	// Queue the members for reconciliation
	for _, m := range me.Members {
		// Change the status if this is a reap event
		if isReap {
			m.Status = StatusReap
		}
		select {
		case s.reconcileCh <- m:
		default:
		}
	}
}
