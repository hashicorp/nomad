//+build linux,lxc

package lxc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/plugins/drivers"
	lxc "gopkg.in/lxc/go-lxc.v2"
)

type taskHandle struct {
	container *lxc.Container
	initPid   int
	logger    hclog.Logger

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats

	// stateLock syncs access to all fields below
	stateLock sync.RWMutex

	taskConfig  *drivers.TaskConfig
	procState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult
}

var (
	LXCMeasuredCpuStats = []string{"System Mode", "User Mode", "Percent"}

	LXCMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage", "Kernel Usage", "Kernel Max Usage"}
)

func (h *taskHandle) TaskStatus() *drivers.TaskStatus {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	return &drivers.TaskStatus{
		ID:          h.taskConfig.ID,
		Name:        h.taskConfig.Name,
		State:       h.procState,
		StartedAt:   h.startedAt,
		CompletedAt: h.completedAt,
		ExitResult:  h.exitResult,
		DriverAttributes: map[string]string{
			"pid": strconv.Itoa(h.initPid),
		},
	}
}

func (h *taskHandle) IsRunning() bool {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()
	return h.procState == drivers.TaskStateRunning
}

func (h *taskHandle) run() {
	h.stateLock.Lock()
	if h.exitResult == nil {
		h.exitResult = &drivers.ExitResult{}
	}
	h.stateLock.Unlock()

	if ok, err := waitTillStopped(h.container); !ok {
		h.logger.Error("failed to find container process", "error", err)
		return
	}

	h.stateLock.Lock()
	defer h.stateLock.Unlock()

	h.procState = drivers.TaskStateExited
	h.exitResult.ExitCode = 0
	h.exitResult.Signal = 0
	h.completedAt = time.Now()

	// TODO: detect if the task OOMed
}

func (h *taskHandle) stats() (*drivers.TaskResourceUsage, error) {
	cpuStats, err := h.container.CPUStats()
	if err != nil {
		h.logger.Error("failed to get container cpu stats", "error", err)
		return nil, nil
	}
	total, err := h.container.CPUTime()
	if err != nil {
		h.logger.Error("failed to get container cpu time", "error", err)
		return nil, nil
	}

	t := time.Now()

	// Get the cpu stats
	system := cpuStats["system"]
	user := cpuStats["user"]
	cs := &drivers.CpuStats{
		SystemMode: h.systemCpuStats.Percent(float64(system)),
		UserMode:   h.systemCpuStats.Percent(float64(user)),
		Percent:    h.totalCpuStats.Percent(float64(total)),
		TotalTicks: float64(user + system),
		Measured:   LXCMeasuredCpuStats,
	}

	// Get the Memory Stats
	memData := map[string]uint64{
		"rss":   0,
		"cache": 0,
		"swap":  0,
	}
	rawMemStats := h.container.CgroupItem("memory.stat")
	for _, rawMemStat := range rawMemStats {
		key, val, err := keysToVal(rawMemStat)
		if err != nil {
			h.logger.Error("failed to get stat", "line", rawMemStat, "error", err)
			continue
		}
		if _, ok := memData[key]; ok {
			memData[key] = val

		}
	}
	ms := &drivers.MemoryStats{
		RSS:      memData["rss"],
		Cache:    memData["cache"],
		Swap:     memData["swap"],
		Measured: LXCMeasuredMemStats,
	}

	mu := h.container.CgroupItem("memory.max_usage_in_bytes")
	for _, rawMemMaxUsage := range mu {
		val, err := strconv.ParseUint(rawMemMaxUsage, 10, 64)
		if err != nil {
			h.logger.Error("failed to get max memory usage", "error", err)
			continue
		}
		ms.MaxUsage = val
	}
	ku := h.container.CgroupItem("memory.kmem.usage_in_bytes")
	for _, rawKernelUsage := range ku {
		val, err := strconv.ParseUint(rawKernelUsage, 10, 64)
		if err != nil {
			h.logger.Error("failed to get kernel memory usage", "error", err)
			continue
		}
		ms.KernelUsage = val
	}

	mku := h.container.CgroupItem("memory.kmem.max_usage_in_bytes")
	for _, rawMaxKernelUsage := range mku {
		val, err := strconv.ParseUint(rawMaxKernelUsage, 10, 64)
		if err != nil {
			h.logger.Error("failed tog get max kernel memory usage", "error", err)
			continue
		}
		ms.KernelMaxUsage = val
	}

	taskResUsage := drivers.TaskResourceUsage{
		ResourceUsage: &drivers.ResourceUsage{
			CpuStats:    cs,
			MemoryStats: ms,
		},
		Timestamp: t.UTC().UnixNano(),
	}

	return &taskResUsage, nil

}

func keysToVal(line string) (string, uint64, error) {
	tokens := strings.Split(line, " ")
	if len(tokens) != 2 {
		return "", 0, fmt.Errorf("line isn't a k/v pair")
	}
	key := tokens[0]
	val, err := strconv.ParseUint(tokens[1], 10, 64)
	return key, val, err
}

// shutdown shuts down the container, with `timeout` grace period
// before killing the container with SIGKILL.
func (h *taskHandle) shutdown(timeout time.Duration) error {
	err := h.container.Shutdown(timeout)
	if err == nil {
		return nil
	}

	return h.container.Stop()
}
