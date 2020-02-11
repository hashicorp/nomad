package raft

import "context"

type LeadershipTransitionDelegate interface {
	GainedLeadership(ctx context.Context) error
	LostLeadership() error
	Heartbeat() error
}
