// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package procstats

import (
	"testing"
	"time"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/shoenig/test/must"
	"oss.indeed.com/go/libtime"
)

type mockPL struct{}

func (mockPL) ListProcesses() set.Collection[ProcessID] { return set.New[ProcessID](0) }

func TestStatProcesses(t *testing.T) {
	compute := cpustats.Compute{
		TotalCompute: 1000,
		NumCores:     1,
	}
	pl := mockPL{}

	stats := &taskProcStats{
		cacheTTL: 10 * time.Second,
		procList: pl,
		compute:  compute,
		clock:    libtime.SystemClock(),
		latest:   make(map[ProcessID]*stats),
		cache:    make(ProcUsages),
	}

	stats.StatProcesses()
	cachedAt := stats.at
	must.NotEq(t, time.Time{}, cachedAt)
	stats.StatProcesses()
	must.Eq(t, cachedAt, stats.at, must.Sprint("cache should not have been updated"))
}
