package config

import (
	"time"

	"github.com/hashicorp/nomad/helper"
)

type AutopilotConfig struct {
	// CleanupDeadServers controls whether to remove dead servers when a new
	// server is added to the Raft peers.
	CleanupDeadServers *bool `mapstructure:"cleanup_dead_servers"`

	// ServerStabilizationTime is the minimum amount of time a server must be
	// in a stable, healthy state before it can be added to the cluster. Only
	// applicable with Raft protocol version 3 or higher.
	ServerStabilizationTime time.Duration `mapstructure:"server_stabilization_time"`

	// LastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	LastContactThreshold time.Duration `mapstructure:"last_contact_threshold"`

	// MaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy.
	MaxTrailingLogs int `mapstructure:"max_trailing_logs"`

	// (Enterprise-only) EnableRedundancyZones specifies whether to enable redundancy zones.
	EnableRedundancyZones *bool `mapstructure:"enable_redundancy_zones"`

	// (Enterprise-only) DisableUpgradeMigration will disable Autopilot's upgrade migration
	// strategy of waiting until enough newer-versioned servers have been added to the
	// cluster before promoting them to voters.
	DisableUpgradeMigration *bool `mapstructure:"disable_upgrade_migration"`

	// (Enterprise-only) EnableCustomUpgrades specifies whether to enable using custom
	// upgrade versions when performing migrations.
	EnableCustomUpgrades *bool `mapstructure:"enable_custom_upgrades"`
}

// DefaultAutopilotConfig() returns the canonical defaults for the Nomad
// `autopilot` configuration.
func DefaultAutopilotConfig() *AutopilotConfig {
	return &AutopilotConfig{
		LastContactThreshold:    200 * time.Millisecond,
		MaxTrailingLogs:         250,
		ServerStabilizationTime: 10 * time.Second,
	}
}

func (a *AutopilotConfig) Merge(b *AutopilotConfig) *AutopilotConfig {
	result := a.Copy()

	if b.CleanupDeadServers != nil {
		result.CleanupDeadServers = helper.BoolToPtr(*b.CleanupDeadServers)
	}
	if b.ServerStabilizationTime != 0 {
		result.ServerStabilizationTime = b.ServerStabilizationTime
	}
	if b.LastContactThreshold != 0 {
		result.LastContactThreshold = b.LastContactThreshold
	}
	if b.MaxTrailingLogs != 0 {
		result.MaxTrailingLogs = b.MaxTrailingLogs
	}
	if b.EnableRedundancyZones != nil {
		result.EnableRedundancyZones = b.EnableRedundancyZones
	}
	if b.DisableUpgradeMigration != nil {
		result.DisableUpgradeMigration = helper.BoolToPtr(*b.DisableUpgradeMigration)
	}
	if b.EnableCustomUpgrades != nil {
		result.EnableCustomUpgrades = b.EnableCustomUpgrades
	}

	return result
}

// Copy returns a copy of this Autopilot config.
func (a *AutopilotConfig) Copy() *AutopilotConfig {
	if a == nil {
		return nil
	}

	nc := new(AutopilotConfig)
	*nc = *a

	// Copy the bools
	if a.CleanupDeadServers != nil {
		nc.CleanupDeadServers = helper.BoolToPtr(*a.CleanupDeadServers)
	}
	if a.EnableRedundancyZones != nil {
		nc.EnableRedundancyZones = helper.BoolToPtr(*a.EnableRedundancyZones)
	}
	if a.DisableUpgradeMigration != nil {
		nc.DisableUpgradeMigration = helper.BoolToPtr(*a.DisableUpgradeMigration)
	}
	if a.EnableCustomUpgrades != nil {
		nc.EnableCustomUpgrades = helper.BoolToPtr(*a.EnableCustomUpgrades)
	}

	return nc
}
