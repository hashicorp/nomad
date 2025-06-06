// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package status

/*
 * This package containst only statuses and descriptions used across the whole
 * scheduler package.
 */

const (
	// AllocNotNeeded is the status used when a job no longer requires an allocation
	AllocNotNeeded = "alloc not needed due to job update"

	// AllocReconnected is the status to use when a replacement allocation is stopped
	// because a disconnected node reconnects.
	AllocReconnected = "alloc not needed due to disconnected client reconnect"

	// AllocMigrating is the status used when we must migrate an allocation
	AllocMigrating = "alloc is being migrated"

	// AllocUpdating is the status used when a job requires an update
	AllocUpdating = "alloc is being updated due to job update"

	// AllocLost is the status used when an allocation is lost
	AllocLost = "alloc is lost since its node is down"

	// AllocUnknown is the status used when an allocation is unknown
	AllocUnknown = "alloc is unknown since its node is disconnected"

	// AllocInPlace is the status used when speculating on an in-place update
	AllocInPlace = "alloc updating in-place"

	// AllocNodeTainted is the status used when stopping an alloc because its
	// node is tainted.
	AllocNodeTainted = "alloc not needed as node is tainted"

	// AllocRescheduled is the status used when an allocation failed and was rescheduled
	AllocRescheduled = "alloc was rescheduled because it failed"

	// BlockedEvalMaxPlanDesc is the description used for blocked evals that are
	// a result of hitting the max number of plan attempts
	BlockedEvalMaxPlanDesc = "created due to placement conflicts"

	// BlockedEvalFailedPlacements is the description used for blocked evals
	// that are a result of failing to place all allocations.
	BlockedEvalFailedPlacements = "created to place remaining allocations"

	// ReschedulingFollowupEvalDesc is the description used when creating follow
	// up evals for delayed rescheduling
	ReschedulingFollowupEvalDesc = "created for delayed rescheduling"

	// DisconnectTimeoutFollowupEvalDesc is the description used when creating follow
	// up evals for allocations that be should be stopped after its disconnect
	// timeout has passed.
	DisconnectTimeoutFollowupEvalDesc = "created for delayed disconnect timeout"
)
