package nomad

import "github.com/hashicorp/serf/serf"

// serfEventHandler is used to handle events from the serf cluster
func (s *Server) serfEventHandler() {
	for {
		select {
		case e := <-s.eventCh:
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.nodeJoin(e.(serf.MemberEvent))
			case serf.EventMemberLeave, serf.EventMemberFailed:
				s.nodeFailed(e.(serf.MemberEvent))
			case serf.EventMemberUpdate, serf.EventMemberReap,
				serf.EventUser, serf.EventQuery: // Ignore
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
		s.peerLock.Unlock()

		// TODO: Add as Raft peer

		// If we still expecting to bootstrap, may need to handle this
		if s.config.BootstrapExpect != 0 {
			s.maybeBootstrap()
		}
	}
}

// maybeBootsrap is used to handle bootstrapping when a new consul server joins
func (s *Server) maybeBootstrap() {
	//index, err := s.raftStore.LastIndex()
	//if err != nil {
	//    s.logger.Printf("[ERR] consul: failed to read last raft index: %v", err)
	//    return
	//}

	//// Bootstrap can only be done if there are no committed logs,
	//// remove our expectations of bootstrapping
	//if index != 0 {
	//    s.config.BootstrapExpect = 0
	//    return
	//}

	//// Scan for all the known servers
	//members := s.serfLAN.Members()
	//addrs := make([]string, 0)
	//for _, member := range members {
	//    valid, p := isConsulServer(member)
	//    if !valid {
	//        continue
	//    }
	//    if p.Datacenter != s.config.Datacenter {
	//        s.logger.Printf("[ERR] consul: Member %v has a conflicting datacenter, ignoring", member)
	//        continue
	//    }
	//    if p.Expect != 0 && p.Expect != s.config.BootstrapExpect {
	//        s.logger.Printf("[ERR] consul: Member %v has a conflicting expect value. All nodes should expect the same number.", member)
	//        return
	//    }
	//    if p.Bootstrap {
	//        s.logger.Printf("[ERR] consul: Member %v has bootstrap mode. Expect disabled.", member)
	//        return
	//    }
	//    addr := &net.TCPAddr{IP: member.Addr, Port: p.Port}
	//    addrs = append(addrs, addr.String())
	//}

	//// Skip if we haven't met the minimum expect count
	//if len(addrs) < s.config.BootstrapExpect {
	//    return
	//}

	//// Update the peer set
	//s.logger.Printf("[INFO] consul: Attempting bootstrap with nodes: %v", addrs)
	//if err := s.raft.SetPeers(addrs).Error(); err != nil {
	//    s.logger.Printf("[ERR] consul: failed to bootstrap peers: %v", err)
	//}

	//// Bootstrapping comlete, don't enter this again
	//s.config.BootstrapExpect = 0
}

// nodeFailed is used to handle fail events on the serf cluster
func (s *Server) nodeFailed(me serf.MemberEvent) {
	//for _, m := range me.Members {
	//    ok, parts := isConsulServer(m)
	//    if !ok {
	//        continue
	//    }
	//    s.logger.Printf("[INFO] consul: removing server %s", parts)

	//    // Remove the server if known
	//    s.remoteLock.Lock()
	//    existing := s.remoteConsuls[parts.Datacenter]
	//    n := len(existing)
	//    for i := 0; i < n; i++ {
	//        if existing[i].Name == parts.Name {
	//            existing[i], existing[n-1] = existing[n-1], nil
	//            existing = existing[:n-1]
	//            n--
	//            break
	//        }
	//    }

	//    // Trim the list if all known consuls are dead
	//    if n == 0 {
	//        delete(s.remoteConsuls, parts.Datacenter)
	//    } else {
	//        s.remoteConsuls[parts.Datacenter] = existing
	//    }
	//    s.remoteLock.Unlock()

	//    // Remove from the local list as well
	//    if !wan {
	//        s.localLock.Lock()
	//        delete(s.localConsuls, parts.Addr.String())
	//        s.localLock.Unlock()
	//    }
	//}
}
