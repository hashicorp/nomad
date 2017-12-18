package nomad

import (
	"context"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// AutopilotDelegate is a Consul delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

func (d *AutopilotDelegate) AutopilotConfig() *autopilot.Config {
	return d.server.getOrCreateAutopilotConfig()
}

func (d *AutopilotDelegate) FetchStats(ctx context.Context, servers []serf.Member) map[string]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

func (d *AutopilotDelegate) IsServer(m serf.Member) (*autopilot.ServerInfo, error) {
	ok, parts := isNomadServer(m)
	if !ok || parts.Region != d.server.Region() {
		return nil, nil
	}

	server := &autopilot.ServerInfo{
		Name:   m.Name,
		ID:     parts.ID,
		Addr:   parts.Addr,
		Build:  parts.Build,
		Status: m.Status,
	}
	return server, nil
}

// Heartbeat a metric for monitoring if we're the leader
func (d *AutopilotDelegate) NotifyHealth(health autopilot.OperatorHealthReply) {
	if d.server.raft.State() == raft.Leader {
		metrics.SetGauge([]string{"nomad", "autopilot", "failure_tolerance"}, float32(health.FailureTolerance))
		if health.Healthy {
			metrics.SetGauge([]string{"nomad", "autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"nomad", "autopilot", "healthy"}, 0)
		}
	}
}

func (d *AutopilotDelegate) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	future := d.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get raft configuration: %v", err)
	}

	return autopilot.PromoteStableServers(conf, health, future.Configuration().Servers), nil
}

func (d *AutopilotDelegate) Raft() *raft.Raft {
	return d.server.raft
}

func (d *AutopilotDelegate) Serf() *serf.Serf {
	return d.server.serf
}
