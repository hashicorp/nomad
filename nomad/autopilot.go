// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"fmt"
	"strconv"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"
)

const (
	// AutopilotRZTag is the Serf tag to use for the redundancy zone value
	// when passing the server metadata to Autopilot.
	AutopilotRZTag = "ap_zone"

	// AutopilotRZTag is the Serf tag to use for the custom version value
	// when passing the server metadata to Autopilot.
	AutopilotVersionTag = "ap_version"
)

// AutopilotDelegate is a Nomad delegate for autopilot operations. It implements
// the autopilot.ApplicationIntegration interface, and the methods required for
// that interface have been documented as such below.
type AutopilotDelegate struct {
	server *Server
}

// AutopilotConfig is used to retrieve the latest configuration from the Nomad
// delegate. This method is required to implement the ApplicationIntegration
// interface.
func (d *AutopilotDelegate) AutopilotConfig() *autopilot.Config {
	c := d.server.getOrCreateAutopilotConfig()
	if c == nil {
		return nil
	}
	conf := &autopilot.Config{
		CleanupDeadServers:      c.CleanupDeadServers,
		LastContactThreshold:    c.LastContactThreshold,
		MaxTrailingLogs:         c.MaxTrailingLogs,
		MinQuorum:               c.MinQuorum,
		ServerStabilizationTime: c.ServerStabilizationTime,
		Ext:                     autopilotConfigExt(c),
	}

	return conf
}

// FetchServerStats will be called by autopilot to request Nomad fetch the
// server stats out of band. This method is required to implement the
// ApplicationIntegration interface
func (d *AutopilotDelegate) FetchServerStats(ctx context.Context, servers map[raft.ServerID]*autopilot.Server) map[raft.ServerID]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

// KnownServers will be called by autopilot to request the list of servers known
// to Nomad. This method is required to implement the ApplicationIntegration
// interface
func (d *AutopilotDelegate) KnownServers() map[raft.ServerID]*autopilot.Server {
	return d.server.autopilotServers()
}

// NotifyState will be called when the autopilot state is updated. The Nomad
// leader heartbeats a metric for monitoring based on this information. This
// method is required to implement the ApplicationIntegration interface
func (d *AutopilotDelegate) NotifyState(state *autopilot.State) {
	if d.server.raft.State() == raft.Leader {
		metrics.SetGauge([]string{"nomad", "autopilot", "failure_tolerance"}, float32(state.FailureTolerance))
		if state.Healthy {
			metrics.SetGauge([]string{"nomad", "autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"nomad", "autopilot", "healthy"}, 0)
		}
	}
}

// RemoveFailedServer will be called by autopilot to notify Nomad to remove the
// server in a failed state. This method is required to implement the
// ApplicationIntegration interface. (Note this is expected to return
// immediately so we'll spawn a goroutine for it.)
func (d *AutopilotDelegate) RemoveFailedServer(failedSrv *autopilot.Server) {
	go func() {
		err := d.server.RemoveFailedNode(failedSrv.Name)
		if err != nil {
			d.server.logger.Error("could not remove failed server",
				"server", string(failedSrv.ID),
				"error", err,
			)
		}
	}()
}

// MinRaftProtocol returns the lowest supported Raft protocol among alive
// servers
func (s *Server) MinRaftProtocol() (int, error) {
	return minRaftProtocol(s.serf.Members(), isNomadServer)
}

// GetClusterHealth is used to get the current health of the servers, as known
// by the leader.
func (s *Server) GetClusterHealth() *structs.OperatorHealthReply {

	state := s.autopilot.GetState()
	if state == nil {
		// this behavior seems odd but its functionally equivalent to 1.8.5 where if
		// autopilot didn't have a health reply yet it would just return no error
		return nil
	}

	health := &structs.OperatorHealthReply{
		Healthy:          state.Healthy,
		FailureTolerance: state.FailureTolerance,
	}

	for _, srv := range state.Servers {
		srvHealth := structs.ServerHealth{
			ID:          string(srv.Server.ID),
			Name:        srv.Server.Name,
			Address:     string(srv.Server.Address),
			Version:     srv.Server.Version,
			Leader:      srv.State == autopilot.RaftLeader,
			Voter:       srv.State == autopilot.RaftLeader || srv.State == autopilot.RaftVoter,
			LastContact: srv.Stats.LastContact,
			LastTerm:    srv.Stats.LastTerm,
			LastIndex:   srv.Stats.LastIndex,
			Healthy:     srv.Health.Healthy,
			StableSince: srv.Health.StableSince,
		}

		switch srv.Server.NodeStatus {
		case autopilot.NodeAlive:
			srvHealth.SerfStatus = serf.StatusAlive
		case autopilot.NodeLeft:
			srvHealth.SerfStatus = serf.StatusLeft
		case autopilot.NodeFailed:
			srvHealth.SerfStatus = serf.StatusFailed
		default:
			srvHealth.SerfStatus = serf.StatusNone
		}

		health.Servers = append(health.Servers, srvHealth)
	}

	return health
}

// -------------------
// helper functions

func minRaftProtocol(members []serf.Member, serverFunc func(serf.Member) (bool, *serverParts)) (int, error) {
	minVersion := -1
	for _, m := range members {
		if m.Status != serf.StatusAlive {
			continue
		}

		ok, server := serverFunc(m)
		if !ok {
			return -1, fmt.Errorf("not a Nomad server")
		}
		if server == nil {
			continue
		}

		vsn, ok := m.Tags["raft_vsn"]
		if !ok {
			vsn = "1"
		}
		raftVsn, err := strconv.Atoi(vsn)
		if err != nil {
			return -1, err
		}

		if minVersion == -1 || raftVsn < minVersion {
			minVersion = raftVsn
		}
	}

	if minVersion == -1 {
		return minVersion, fmt.Errorf("No servers found")
	}

	return minVersion, nil
}

func (s *Server) autopilotServers() map[raft.ServerID]*autopilot.Server {
	servers := make(map[raft.ServerID]*autopilot.Server)

	for _, member := range s.serf.Members() {
		srv, err := s.autopilotServer(member)
		if err != nil {
			s.logger.Warn("Error parsing server info", "name", member.Name, "error", err)
			continue
		} else if srv == nil {
			// this member was a client or in another region
			continue
		}

		servers[srv.ID] = srv
	}

	return servers
}

func (s *Server) autopilotServer(m serf.Member) (*autopilot.Server, error) {
	ok, srv := isNomadServer(m)
	if !ok {
		return nil, nil
	}
	if srv.Region != s.Region() {
		return nil, nil
	}

	return s.autopilotServerFromMetadata(srv)
}

func (s *Server) autopilotServerFromMetadata(srv *serverParts) (*autopilot.Server, error) {
	server := &autopilot.Server{
		Name:        srv.Name,
		ID:          raft.ServerID(srv.ID),
		Address:     raft.ServerAddress(srv.Addr.String()),
		Version:     srv.Build.String(),
		RaftVersion: srv.RaftVersion,
		Ext:         s.autopilotServerExt(srv),
	}

	switch srv.Status {
	case serf.StatusLeft:
		server.NodeStatus = autopilot.NodeLeft
	case serf.StatusAlive, serf.StatusLeaving:
		// we want to treat leaving as alive to prevent autopilot from
		// prematurely removing the node.
		server.NodeStatus = autopilot.NodeAlive
	case serf.StatusFailed:
		server.NodeStatus = autopilot.NodeFailed
	default:
		server.NodeStatus = autopilot.NodeUnknown
	}

	members := s.serf.Members()
	for _, member := range members {
		if member.Name == srv.Name {
			server.Meta = member.Tags
			break
		}
	}

	return server, nil
}
