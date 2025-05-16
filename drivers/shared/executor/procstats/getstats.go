// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package procstats

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shirou/gopsutil/v3/process"
	"oss.indeed.com/go/libtime"
)

func New(compute cpustats.Compute, pl ProcessList) ProcessStats {
	const cacheTTL = 5 * time.Second
	return &taskProcStats{
		cacheTTL: cacheTTL,
		procList: pl,
		compute:  compute,
		clock:    libtime.SystemClock(),
		latest:   make(map[ProcessID]*stats),
		cache:    make(ProcUsages),
	}
}

type stats struct {
	TotalCPU  *cpustats.Tracker
	UserCPU   *cpustats.Tracker
	SystemCPU *cpustats.Tracker
}

type taskProcStats struct {
	cacheTTL time.Duration
	procList ProcessList
	clock    libtime.Clock
	compute  cpustats.Compute

	lock   sync.Mutex
	latest map[ProcessID]*stats
	cache  ProcUsages
	at     time.Time
}

func (lps *taskProcStats) expired() bool {
	age := lps.clock.Since(lps.at)
	return age > lps.cacheTTL
}

// scanPIDs will update lps.latest with the set of detected live pids that make
// up the task process tree / are in the tasks cgroup
func (lps *taskProcStats) scanPIDs() {
	currentPIDs := lps.procList.ListProcesses()

	// remove old pids no longer present
	for pid := range lps.latest {
		if !currentPIDs.Contains(pid) {
			delete(lps.latest, pid)
		}
	}

	// insert trackers for new pids not yet present
	for pid := range currentPIDs.Items() {
		if _, exists := lps.latest[pid]; !exists {
			lps.latest[pid] = &stats{
				TotalCPU:  cpustats.New(lps.compute),
				UserCPU:   cpustats.New(lps.compute),
				SystemCPU: cpustats.New(lps.compute),
			}
		}
	}
}

func (lps *taskProcStats) cached() ProcUsages {
	return lps.cache
}

func (lps *taskProcStats) StatProcesses() ProcUsages {
	lps.lock.Lock()
	defer lps.lock.Unlock()

	if !lps.expired() {
		return lps.cache
	}

	// the stats are expired, scan for new information
	lps.scanPIDs()

	// create the response resource usage map
	var result = make(ProcUsages)
	for pid, s := range lps.latest {
		p, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}

		getMemory := func() *drivers.MemoryStats {
			ms := new(drivers.MemoryStats)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if memInfo, err := p.MemoryInfoWithContext(ctx); err == nil {
				ms.RSS = memInfo.RSS
				ms.Swap = memInfo.Swap
				ms.Measured = ExecutorBasicMeasuredMemStats
			}
			return ms
		}

		getCPU := func() *drivers.CpuStats {
			cs := new(drivers.CpuStats)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if cpuInfo, err := p.TimesWithContext(ctx); err == nil {
				const second = float64(time.Second)
				cs.SystemMode = s.SystemCPU.Percent(cpuInfo.System * second)
				cs.UserMode = s.UserCPU.Percent(cpuInfo.User * second)
				cs.Percent = s.TotalCPU.Percent(cpuInfo.Total() * second)
				cs.Measured = ExecutorBasicMeasuredCpuStats
			}
			return cs
		}

		spid := strconv.Itoa(pid)
		result[spid] = &drivers.ResourceUsage{
			MemoryStats: getMemory(),
			CpuStats:    getCPU(),
		}
	}

	lps.cache = result
	lps.at = time.Now()
	return result
}
