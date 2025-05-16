// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package procstats

import (
	"testing"
	"time"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/shoenig/test/must"
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
		latest:   make(map[ProcessID]*stats),
		cache:    make(ProcUsages),
	}

	now := time.Now()
	stats.StatProcesses(now)
	cachedAt := stats.at
	must.NotEq(t, time.Time{}, cachedAt)

	stats.StatProcesses(now)
	must.Eq(t, cachedAt, stats.at, must.Sprint("cache should not have been updated"))

	later := now.Add(30 * time.Second)
	stats.StatProcesses(later)
	must.Eq(t, later, stats.at, must.Sprint("cache should have been updated"))
}
