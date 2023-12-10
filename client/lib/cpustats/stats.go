// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package cpustats provides utilities for tracking CPU usage statistics.
package cpustats

import (
	"time"

	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"oss.indeed.com/go/libtime"
)

// Compute is the cpu related fields of a numa.Topology needed for computing
// performance / utilization of tasks.
//
// Note that this is serialized and passed to executor.Executor as a CLI
// argument and so we use small json field names to minimize ps spam.
type Compute struct {
	TotalCompute hw.MHz `json:"tc"`
	NumCores     int    `json:"nc"`
}

// A Tracker keeps track of one aspect of CPU utilization (i.e. one of system,
// user, or total time).
type Tracker struct {
	prevCPUTime float64
	prevTime    time.Time

	totalCompute hw.MHz
	numCPUs      int

	clock libtime.Clock
}

// New creates a fresh Tracker with no data.
func New(c Compute) *Tracker {
	return &Tracker{
		totalCompute: c.TotalCompute,
		numCPUs:      c.NumCores,
		clock:        libtime.SystemClock(),
	}
}

// Percent calculates the CPU usage percentage based on the current CPU usage
// and the previous CPU usage where usage is given as a time in nanoseconds
// spent using the CPU.
func (t *Tracker) Percent(cpuTime float64) float64 {
	now := t.clock.Now()

	if t.prevCPUTime == 0.0 {
		t.prevCPUTime = cpuTime
		t.prevTime = now
		return 0.0
	}

	timeDelta := now.Sub(t.prevTime).Nanoseconds()
	ret := t.calculatePercent(t.prevCPUTime, cpuTime, timeDelta)
	t.prevCPUTime = cpuTime
	t.prevTime = now
	return ret
}

func (t *Tracker) calculatePercent(t1, t2 float64, timeDelta int64) float64 {
	vDelta := t2 - t1
	if timeDelta <= 0 || vDelta <= 0 {
		return 0.0
	}
	return (vDelta / float64(timeDelta)) * 100.0
}

// TicksConsumed calculates the total bandwidth consumed by the process across
// all system CPU cores (not just the ones available to Nomad or this particular
// process.
func (t *Tracker) TicksConsumed(percent float64) float64 {
	return (percent / 100) * float64(t.totalCompute) / float64(t.numCPUs)
}
