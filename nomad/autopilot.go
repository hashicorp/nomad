package nomad

import (
	"context"
	"fmt"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/raft"
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

// AutopilotDelegate is a Nomad delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

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
		DisableUpgradeMigration: c.DisableUpgradeMigration,
		ModifyIndex:             c.ModifyIndex,
		CreateIndex:             c.CreateIndex,
	}

	if c.EnableRedundancyZones {
		conf.RedundancyZoneTag = AutopilotRZTag
	}
	if c.EnableCustomUpgrades {
		conf.UpgradeVersionTag = AutopilotVersionTag
	}

	return conf
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

// NotifyHealth heartbeats a metric for monitoring if we're the leader.
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

func (d *AutopilotDelegate) SerfLAN() *serf.Serf {
	return d.server.serf
}

func (d *AutopilotDelegate) SerfWAN() *serf.Serf {
	// serf WAN isn't supported in nomad yet
	return nil
}
