// +build !linux

package executor

import (
	"os"
	"time"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"

	"github.com/mitchellh/go-ps"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

func (e *UniversalExecutor) configureChroot() error {
	return nil
}

func DestroyCgroup(groups *cgroupConfig.Cgroup, paths map[string]string, executorPid int) error {
	return nil
}

func (e *UniversalExecutor) removeChrootMounts() error {
	return nil
}

func (e *UniversalExecutor) runAs(userid string) error {
	return nil
}

func (e *UniversalExecutor) applyLimits(pid int) error {
	return nil
}

func (e *UniversalExecutor) configureIsolation() error {
	return nil
}

func (e *UniversalExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	ts := time.Now()
	pidStats, err := e.pidStats()
	if err != nil {
		return nil, err
	}
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

	totalCPU := &cstructs.CpuStats{
		SystemMode: systemModeCPU,
		UserMode:   userModeCPU,
		Percent:    percent,
	}

	totalMemory := &cstructs.MemoryStats{
		RSS:  totalRSS,
		Swap: totalSwap,
	}

	resourceUsage := cstructs.ResourceUsage{
		MemoryStats: totalMemory,
		CpuStats:    totalCPU,
		Timestamp:   ts,
	}
	return &cstructs.TaskResourceUsage{
		ResourceUsage: &resourceUsage,
		Timestamp:     ts,
		Pids:          pidStats,
	}, nil
}

func (e *UniversalExecutor) getAllPids() ([]*nomadPid, error) {
	allProcesses, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return e.scanPids(os.Getpid(), allProcesses)
}
