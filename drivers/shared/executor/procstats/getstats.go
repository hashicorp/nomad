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

func New(top cpustats.Topology, pl ProcessList) ProcessStats {
	const cacheTTL = 5 * time.Second
	return &linuxProcStats{
		cacheTTL: cacheTTL,
		procList: pl,
		top:      top,
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

type linuxProcStats struct {
	cacheTTL time.Duration
	procList ProcessList
	clock    libtime.Clock
	top      cpustats.Topology

	lock   sync.Mutex
	latest map[ProcessID]*stats
	cache  ProcUsages
	at     time.Time
}

func (lps *linuxProcStats) expired() bool {
	age := lps.clock.Since(lps.at)
	return age > lps.cacheTTL
}

// scanPIDs will update lps.latest with the set of detected live pids that make
// up the task process tree / are in the tasks cgroup
func (lps *linuxProcStats) scanPIDs() {
	currentPIDs := lps.procList.ListProcesses()

	// remove old pids no longer present
	for pid := range lps.latest {
		if !currentPIDs.Contains(pid) {
			delete(lps.latest, pid)
		}
	}

	// insert trackers for new pids not yet present
	for _, pid := range currentPIDs.Slice() {
		if _, exists := lps.latest[pid]; !exists {
			lps.latest[pid] = &stats{
				TotalCPU:  cpustats.New(lps.top),
				UserCPU:   cpustats.New(lps.top),
				SystemCPU: cpustats.New(lps.top),
			}
		}
	}
}

func (lps *linuxProcStats) cached() ProcUsages {
	return lps.cache
}

func (lps *linuxProcStats) StatProcesses() ProcUsages {
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
	return result
}
