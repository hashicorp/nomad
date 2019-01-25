package docker

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/stats"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

const (
	// statsCollectorBackoffBaseline is the baseline time for exponential
	// backoff while calling the docker stats api.
	statsCollectorBackoffBaseline = 5 * time.Second

	// statsCollectorBackoffLimit is the limit of the exponential backoff for
	// calling the docker stats api.
	statsCollectorBackoffLimit = 2 * time.Minute
)

// Stats starts collecting stats from the docker daemon and sends them on the
// returned channel.
func (h *taskHandle) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	select {
	case <-h.doneCh:
		return nil, nstructs.NewRecoverableError(fmt.Errorf("container stopped"), false)
	default:
	}
	ch := make(chan *cstructs.TaskResourceUsage, 1)
	go h.collectStats(ctx, ch, interval)
	return ch, nil
}

// collectStats starts collecting resource usage stats of a docker container
func (h *taskHandle) collectStats(ctx context.Context, ch chan *cstructs.TaskResourceUsage, interval time.Duration) {
	defer close(ch)
	// backoff and retry used if the docker stats API returns an error
	var backoff time.Duration
	var retry int
	// loops until doneCh is closed
	for {
		if backoff > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			case <-h.doneCh:
				return
			}
		}
		// make a channel for docker stats structs and start a collector to
		// receive stats from docker and emit nomad stats
		// statsCh will always be closed by docker client.
		statsCh := make(chan *docker.Stats)
		go dockerStatsCollector(ch, statsCh, interval)

		statsOpts := docker.StatsOptions{
			ID:      h.containerID,
			Context: ctx,
			Done:    h.doneCh,
			Stats:   statsCh,
			Stream:  true,
		}

		// Stats blocks until an error has occurred, or doneCh has been closed
		if err := h.client.Stats(statsOpts); err != nil && err != io.ErrClosedPipe {
			// An error occurred during stats collection, retry with backoff
			h.logger.Debug("error collecting stats from container", "error", err)

			// Calculate the new backoff
			backoff = (1 << (2 * uint64(retry))) * statsCollectorBackoffBaseline
			if backoff > statsCollectorBackoffLimit {
				backoff = statsCollectorBackoffLimit
			}
			// Increment retry counter
			retry++
			continue
		}
		// Stats finished either because context was canceled, doneCh was closed
		// or the container stopped. Stop stats collections.
		return
	}
}
func dockerStatsCollector(destCh chan *cstructs.TaskResourceUsage, statsCh <-chan *docker.Stats, interval time.Duration) {
	var resourceUsage *cstructs.TaskResourceUsage

	// hasSentInitialStats is used so as to emit the first stats received from
	// the docker daemon
	var hasSentInitialStats bool

	// timer is used to send nomad status at the specified interval
	timer := time.NewTimer(interval)
	for {
		select {
		case <-timer.C:
			// it is possible for the timer to go off before the first stats
			// has been emitted from docker
			if resourceUsage == nil {
				continue
			}
			// sending to destCh could block, drop this interval if it does
			select {
			case destCh <- resourceUsage:
			default:
				// Backpressure caused missed interval
			}
			timer.Reset(interval)
		case s, ok := <-statsCh:
			// if statsCh is closed stop collection
			if !ok {
				return
			}
			// s should always be set, but check and skip just in case
			if s != nil {
				resourceUsage = dockerStatsToTaskResourceUsage(s)
				// send stats next interation if this is the first time received
				// from docker
				if !hasSentInitialStats {
					timer.Reset(0)
					hasSentInitialStats = true
				}
			}
		}
	}
}

func dockerStatsToTaskResourceUsage(s *docker.Stats) *cstructs.TaskResourceUsage {
	ms := &cstructs.MemoryStats{
		RSS:      s.MemoryStats.Stats.Rss,
		Cache:    s.MemoryStats.Stats.Cache,
		Swap:     s.MemoryStats.Stats.Swap,
		Usage:    s.MemoryStats.Usage,
		MaxUsage: s.MemoryStats.MaxUsage,
		Measured: DockerMeasuredMemStats,
	}

	cs := &cstructs.CpuStats{
		ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
		ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
		Measured:         DockerMeasuredCpuStats,
	}

	// Calculate percentage
	cs.Percent = calculatePercent(
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
		s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, runtime.NumCPU())
	cs.SystemMode = calculatePercent(
		s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, runtime.NumCPU())
	cs.UserMode = calculatePercent(
		s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, runtime.NumCPU())
	cs.TotalTicks = (cs.Percent / 100) * stats.TotalTicksAvailable() / float64(runtime.NumCPU())

	return &cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: ms,
			CpuStats:    cs,
		},
		Timestamp: s.Read.UTC().UnixNano(),
	}
}

func calculatePercent(newSample, oldSample, newTotal, oldTotal uint64, cores int) float64 {
	numerator := newSample - oldSample
	denom := newTotal - oldTotal
	if numerator <= 0 || denom <= 0 {
		return 0.0
	}

	return (float64(numerator) / float64(denom)) * float64(cores) * 100.0
}
