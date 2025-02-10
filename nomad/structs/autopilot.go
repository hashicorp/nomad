// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"time"

	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"
)

// OperatorHealthReply is a representation of the overall health of the cluster
type OperatorHealthReply struct {
	// Healthy is true if all the servers in the cluster are healthy.
	Healthy bool

	// FailureTolerance is the number of healthy servers that could be lost without
	// an outage occurring.
	FailureTolerance int

	// Servers holds the health of each server.
	Servers []ServerHealth

	// The ID of the current leader.
	Leader string

	// List of servers that are voters in the Raft configuration.
	Voters []string

	// ReadReplicas holds the list of servers that are read replicas in the Raft
	// configuration. (Enterprise only)
	ReadReplicas []string `json:",omitempty"`

	// RedundancyZones holds the list of servers in each redundancy zone.
	// (Enterprise only)
	RedundancyZones map[string]AutopilotZone `json:",omitempty"`

	// Upgrade holds the current upgrade status.
	Upgrade *AutopilotUpgrade `json:",omitempty"`

	// The number of servers that could be lost without an outage occurring if
	// all the voters don't fail at once. (Enterprise only)
	OptimisticFailureTolerance int `json:",omitempty"`
}

// ServerHealth is the health (from the leader's point of view) of a server.
type ServerHealth struct {
	// ID is the raft ID of the server.
	ID string

	// Name is the node name of the server.
	Name string

	// Address is the address of the server.
	Address string

	// The status of the SerfHealth check for the server.
	SerfStatus serf.MemberStatus

	// Version is the Nomad version of the server.
	Version string

	// Leader is whether this server is currently the leader.
	Leader bool

	// LastContact is the time since this node's last contact with the leader.
	LastContact time.Duration

	// LastTerm is the highest leader term this server has a record of in its Raft log.
	LastTerm uint64

	// LastIndex is the last log index this server has a record of in its Raft log.
	LastIndex uint64

	// Healthy is whether or not the server is healthy according to the current
	// Autopilot config.
	Healthy bool

	// Voter is whether this is a voting server.
	Voter bool

	// StableSince is the last time this server's Healthy value changed.
	StableSince time.Time
}

// AutopilotZone holds the list of servers in a redundancy zone.  (Enterprise only)
type AutopilotZone struct {
	// Servers holds the list of servers in the redundancy zone.
	Servers []string

	// Voters holds the list of servers that are voters in the redundancy zone.
	Voters []string

	// FailureTolerance is the number of servers that could be lost without an
	// outage occurring.
	FailureTolerance int
}

// AutopilotUpgrade holds the current upgrade status. (Enterprise only)
type AutopilotUpgrade struct {
	// Status of the upgrade.
	Status string

	// TargetVersion is the version that the cluster is upgrading to.
	TargetVersion string

	// TargetVersionVoters holds the list of servers that are voters in the Raft
	// configuration of the TargetVersion.
	TargetVersionVoters []string

	// TargetVersionNonVoters holds the list of servers that are non-voters in
	// the Raft configuration of the TargetVersion.
	TargetVersionNonVoters []string

	// TargetVersionReadReplicas holds the list of servers that are read
	// replicas in the Raft configuration of the TargetVersion.
	TargetVersionReadReplicas []string

	// OtherVersionVoters holds the list of servers that are voters in the Raft
	// configuration of a version other than the TargetVersion.
	OtherVersionVoters []string

	// OtherVersionNonVoters holds the list of servers that are non-voters in
	// the Raft configuration of a version other than the TargetVersion.
	OtherVersionNonVoters []string

	// OtherVersionReadReplicas holds the list of servers that are read replicas
	// in the Raft configuration of a version other than the TargetVersion.
	OtherVersionReadReplicas []string

	// RedundancyZones holds the list of servers in each redundancy zone for the
	// TargetVersion.
	RedundancyZones map[string]AutopilotZoneUpgradeVersions
}

// AutopilotZoneUpgradeVersions holds the list of servers in a redundancy zone
// for a specific version. (Enterprise only)
type AutopilotZoneUpgradeVersions struct {
	TargetVersionVoters    []string
	TargetVersionNonVoters []string
	OtherVersionVoters     []string
	OtherVersionNonVoters  []string
}

// RaftStats holds miscellaneous Raft metrics for a server, used by autopilot.
type RaftStats struct {
	// LastContact is the time since this node's last contact with the leader.
	LastContact string

	// LastTerm is the highest leader term this server has a record of in its Raft log.
	LastTerm uint64

	// LastIndex is the last log index this server has a record of in its Raft log.
	LastIndex uint64
}

func (s *RaftStats) ToAutopilotServerStats() *autopilot.ServerStats {
	duration, _ := time.ParseDuration(s.LastContact)
	return &autopilot.ServerStats{
		LastContact: duration,
		LastTerm:    s.LastTerm,
		LastIndex:   s.LastIndex,
	}
}
