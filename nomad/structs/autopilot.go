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
