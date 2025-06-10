// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

const (
	// StatusAllocNotNeeded is the status used when a job no longer requires an
	// allocation
	StatusAllocNotNeeded = "alloc not needed due to job update"

	// StatusAllocReconnected is the status to use when a replacement allocation is
	// stopped because a disconnected node reconnects.
	StatusAllocReconnected = "alloc not needed due to disconnected client reconnect"

	// StatusAllocMigrating is the status used when we must migrate an allocation
	StatusAllocMigrating = "alloc is being migrated"

	// StatusAllocUpdating is the status used when a job requires an update
	StatusAllocUpdating = "alloc is being updated due to job update"

	// StatusAllocLost is the status used when an allocation is lost
	StatusAllocLost = "alloc is lost since its node is down"

	// StatusAllocUnknown is the status used when an allocation is unknown
	StatusAllocUnknown = "alloc is unknown since its node is disconnected"

	// StatusAllocInPlace is the status used when speculating on an in-place update
	StatusAllocInPlace = "alloc updating in-place"

	// StatusAllocNodeTainted is the status used when stopping an alloc because its
	// node is tainted.
	StatusAllocNodeTainted = "alloc not needed as node is tainted"

	// StatusAllocRescheduled is the status used when an allocation failed and was
	// rescheduled
	StatusAllocRescheduled = "alloc was rescheduled because it failed"

	// DescBlockedEvalMaxPlan is the description used for blocked evals that are
	// a result of hitting the max number of plan attempts
	DescBlockedEvalMaxPlan = "created due to placement conflicts"

	// DescBlockedEvalFailedPlacements is the description used for blocked evals
	// that are a result of failing to place all allocations.
	DescBlockedEvalFailedPlacements = "created to place remaining allocations"

	// DescReschedulingFollowupEval is the description used when creating follow
	// up evals for delayed rescheduling
	DescReschedulingFollowupEval = "created for delayed rescheduling"

	// DescDisconnectTimeoutFollowupEval is the description used when creating follow
	// up evals for allocations that be should be stopped after its disconnect
	// timeout has passed.
	DescDisconnectTimeoutFollowupEval = "created for delayed disconnect timeout"
)
