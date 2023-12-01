// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

// The structs and helpers in this file are split out of reconciler.go for code
// manageability and should not be shared to the system schedulers! If you need
// something here for system/sysbatch jobs, double-check it's safe to use for
// all scheduler types before moving it into util.go

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// placementResult is an allocation that must be placed. It potentially has a
// previous allocation attached to it that should be stopped only if the
// paired placement is complete. This gives an atomic place/stop behavior to
// prevent an impossible resource ask as part of a rolling update to wipe the
// job out.
type placementResult interface {
	// TaskGroup returns the task group the placement is for
	TaskGroup() *structs.TaskGroup

	// Name returns the name of the desired allocation
	Name() string

	// Canary returns whether the placement should be a canary
	Canary() bool

	// PreviousAllocation returns the previous allocation
	PreviousAllocation() *structs.Allocation

	// IsRescheduling returns whether the placement was rescheduling a failed allocation
	IsRescheduling() bool

	// StopPreviousAlloc returns whether the previous allocation should be
	// stopped and if so the status description.
	StopPreviousAlloc() (bool, string)

	// PreviousLost is true if the previous allocation was lost.
	PreviousLost() bool

	// DowngradeNonCanary indicates that placement should use the latest stable job
	// with the MinJobVersion, rather than the current deployment version
	DowngradeNonCanary() bool

	MinJobVersion() uint64
}

// allocStopResult contains the information required to stop a single allocation
type allocStopResult struct {
	alloc             *structs.Allocation
	clientStatus      string
	statusDescription string
	followupEvalID    string
}

// allocPlaceResult contains the information required to place a single
// allocation
type allocPlaceResult struct {
	name          string
	canary        bool
	taskGroup     *structs.TaskGroup
	previousAlloc *structs.Allocation
	reschedule    bool
	lost          bool

	downgradeNonCanary bool
	minJobVersion      uint64
}

func (a allocPlaceResult) TaskGroup() *structs.TaskGroup           { return a.taskGroup }
func (a allocPlaceResult) Name() string                            { return a.name }
func (a allocPlaceResult) Canary() bool                            { return a.canary }
func (a allocPlaceResult) PreviousAllocation() *structs.Allocation { return a.previousAlloc }
func (a allocPlaceResult) IsRescheduling() bool                    { return a.reschedule }
func (a allocPlaceResult) StopPreviousAlloc() (bool, string)       { return false, "" }
func (a allocPlaceResult) DowngradeNonCanary() bool                { return a.downgradeNonCanary }
func (a allocPlaceResult) MinJobVersion() uint64                   { return a.minJobVersion }
func (a allocPlaceResult) PreviousLost() bool                      { return a.lost }

// allocDestructiveResult contains the information required to do a destructive
// update. Destructive changes should be applied atomically, as in the old alloc
// is only stopped if the new one can be placed.
type allocDestructiveResult struct {
	placeName             string
	placeTaskGroup        *structs.TaskGroup
	stopAlloc             *structs.Allocation
	stopStatusDescription string
}

func (a allocDestructiveResult) TaskGroup() *structs.TaskGroup           { return a.placeTaskGroup }
func (a allocDestructiveResult) Name() string                            { return a.placeName }
func (a allocDestructiveResult) Canary() bool                            { return false }
func (a allocDestructiveResult) PreviousAllocation() *structs.Allocation { return a.stopAlloc }
func (a allocDestructiveResult) IsRescheduling() bool                    { return false }
func (a allocDestructiveResult) StopPreviousAlloc() (bool, string) {
	return true, a.stopStatusDescription
}
func (a allocDestructiveResult) DowngradeNonCanary() bool { return false }
func (a allocDestructiveResult) MinJobVersion() uint64    { return 0 }
func (a allocDestructiveResult) PreviousLost() bool       { return false }
