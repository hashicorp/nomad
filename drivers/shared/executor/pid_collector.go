package executor

import (
	"os"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/plugins/drivers"
	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"
)

var (
	// pidScanInterval is the interval at which the executor scans the process
	// tree for finding out the pids that the executor and it's child processes
	// have forked
	pidScanInterval = 5 * time.Second
)

// pidCollector is a utility that can be embedded in an executor to collect pid
// stats
type pidCollector struct {
	pids    map[int]*nomadPid
	pidLock sync.RWMutex
	logger  hclog.Logger
}

// nomadPid holds a pid and it's cpu percentage calculator
type nomadPid struct {
	pid           int
	cpuStatsTotal *stats.CpuStats
	cpuStatsUser  *stats.CpuStats
	cpuStatsSys   *stats.CpuStats
}

// allPidGetter is a func which is used by the pid collector to gather
// stats on
type allPidGetter func() (map[int]*nomadPid, error)

func newPidCollector(logger hclog.Logger) *pidCollector {
	return &pidCollector{
		pids:   make(map[int]*nomadPid),
		logger: logger.Named("pid_collector"),
	}
}

// collectPids collects the pids of the child processes that the executor is
// running every 5 seconds
func (c *pidCollector) collectPids(stopCh chan interface{}, pidGetter allPidGetter) {
	// Fire the timer right away when the executor starts from there on the pids
	// are collected every scan interval
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			pids, err := pidGetter()
			if err != nil {
				c.logger.Debug("error collecting pids", "error", err)
			}
			c.pidLock.Lock()

			// Adding pids which are not being tracked
			for pid, np := range pids {
				if _, ok := c.pids[pid]; !ok {
					c.pids[pid] = np
				}
			}
			// Removing pids which are no longer present
			for pid := range c.pids {
				if _, ok := pids[pid]; !ok {
					delete(c.pids, pid)
				}
			}
			c.pidLock.Unlock()
			timer.Reset(pidScanInterval)
		case <-stopCh:
			return
		}
	}
}

// scanPids scans all the pids on the machine running the current executor and
// returns the child processes of the executor.
func scanPids(parentPid int, allPids []ps.Process) (map[int]*nomadPid, error) {
	processFamily := make(map[int]struct{})
	processFamily[parentPid] = struct{}{}

	// A mapping of pids to their parent pids. It is used to build the process
	// tree of the executing task
	pidsRemaining := make(map[int]int, len(allPids))
	for _, pid := range allPids {
		pidsRemaining[pid.Pid()] = pid.PPid()
	}

	for {
		// flag to indicate if we have found a match
		foundNewPid := false

		for pid, ppid := range pidsRemaining {
			_, childPid := processFamily[ppid]

			// checking if the pid is a child of any of the parents
			if childPid {
				processFamily[pid] = struct{}{}
				delete(pidsRemaining, pid)
				foundNewPid = true
			}
		}

		// not scanning anymore if we couldn't find a single match
		if !foundNewPid {
			break
		}
	}

	res := make(map[int]*nomadPid)
	for pid := range processFamily {
		np := nomadPid{
			pid:           pid,
			cpuStatsTotal: stats.NewCpuStats(),
			cpuStatsUser:  stats.NewCpuStats(),
			cpuStatsSys:   stats.NewCpuStats(),
		}
		res[pid] = &np
	}
	return res, nil
}

// pidStats returns the resource usage stats per pid
func (c *pidCollector) pidStats() (map[string]*drivers.ResourceUsage, error) {
	stats := make(map[string]*drivers.ResourceUsage)
	c.pidLock.RLock()
	pids := make(map[int]*nomadPid, len(c.pids))
	for k, v := range c.pids {
		pids[k] = v
	}
	c.pidLock.RUnlock()
	for pid, np := range pids {
		p, err := process.NewProcess(int32(pid))
		if err != nil {
			c.logger.Trace("unable to create new process", "pid", pid, "error", err)
			continue
		}
		ms := &drivers.MemoryStats{}
		if memInfo, err := p.MemoryInfo(); err == nil {
			ms.RSS = memInfo.RSS
			ms.Swap = memInfo.Swap
			ms.Measured = ExecutorBasicMeasuredMemStats
		}

		cs := &drivers.CpuStats{}
		if cpuStats, err := p.Times(); err == nil {
			cs.SystemMode = np.cpuStatsSys.Percent(cpuStats.System * float64(time.Second))
			cs.UserMode = np.cpuStatsUser.Percent(cpuStats.User * float64(time.Second))
			cs.Measured = ExecutorBasicMeasuredCpuStats

			// calculate cpu usage percent
			cs.Percent = np.cpuStatsTotal.Percent(cpuStats.Total() * float64(time.Second))
		}
		stats[strconv.Itoa(pid)] = &drivers.ResourceUsage{MemoryStats: ms, CpuStats: cs}
	}

	return stats, nil
}

// aggregatedResourceUsage aggregates the resource usage of all the pids and
// returns a TaskResourceUsage data point
func aggregatedResourceUsage(systemCpuStats *stats.CpuStats, pidStats map[string]*drivers.ResourceUsage) *drivers.TaskResourceUsage {
	ts := time.Now().UTC().UnixNano()
	var (
		systemModeCPU, userModeCPU, percent float64
		totalRSS, totalSwap                 uint64
	)

	for _, pidStat := range pidStats {
		systemModeCPU += pidStat.CpuStats.SystemMode
		userModeCPU += pidStat.CpuStats.UserMode
		percent += pidStat.CpuStats.Percent

		totalRSS += pidStat.MemoryStats.RSS
		totalSwap += pidStat.MemoryStats.Swap
	}

	totalCPU := &drivers.CpuStats{
		SystemMode: systemModeCPU,
		UserMode:   userModeCPU,
		Percent:    percent,
		Measured:   ExecutorBasicMeasuredCpuStats,
		TotalTicks: systemCpuStats.TicksConsumed(percent),
	}

	totalMemory := &drivers.MemoryStats{
		RSS:      totalRSS,
		Swap:     totalSwap,
		Measured: ExecutorBasicMeasuredMemStats,
	}

	resourceUsage := drivers.ResourceUsage{
		MemoryStats: totalMemory,
		CpuStats:    totalCPU,
	}
	return &drivers.TaskResourceUsage{
		ResourceUsage: &resourceUsage,
		Timestamp:     ts,
		Pids:          pidStats,
	}
}

func getAllPidsByScanning() (map[int]*nomadPid, error) {
	allProcesses, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return scanPids(os.Getpid(), allProcesses)
}
