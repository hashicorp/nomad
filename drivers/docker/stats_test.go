package docker

import (
	"runtime"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func TestDriver_DockerStatsCollector(t *testing.T) {
	require := require.New(t)
	src := make(chan *docker.Stats)
	dst := make(chan *drivers.TaskResourceUsage)

	stats := &docker.Stats{}
	stats.CPUStats.ThrottlingData.Periods = 10
	stats.CPUStats.ThrottlingData.ThrottledPeriods = 10
	stats.CPUStats.ThrottlingData.ThrottledTime = 10

	stats.MemoryStats.Stats.Rss = 6537216
	stats.MemoryStats.Stats.Cache = 1234
	stats.MemoryStats.Stats.Swap = 0
	stats.MemoryStats.Usage = 5651904
	stats.MemoryStats.MaxUsage = 6651904
	stats.MemoryStats.Commit = 123231
	stats.MemoryStats.CommitPeak = 321323
	stats.MemoryStats.PrivateWorkingSet = 62222

	go dockerStatsCollector(dst, src, time.Second)

	select {
	case src <- stats:
	case <-time.After(time.Second):
		require.Fail("sending stats should not block here")
	}

	select {
	case ru := <-dst:
		if runtime.GOOS != "windows" {
			require.Equal(stats.MemoryStats.Stats.Rss, ru.ResourceUsage.MemoryStats.RSS)
			require.Equal(stats.MemoryStats.Stats.Cache, ru.ResourceUsage.MemoryStats.Cache)
			require.Equal(stats.MemoryStats.Stats.Swap, ru.ResourceUsage.MemoryStats.Swap)
			require.Equal(stats.MemoryStats.Usage, ru.ResourceUsage.MemoryStats.Usage)
			require.Equal(stats.MemoryStats.MaxUsage, ru.ResourceUsage.MemoryStats.MaxUsage)
			require.Equal(stats.CPUStats.ThrottlingData.ThrottledPeriods, ru.ResourceUsage.CpuStats.ThrottledPeriods)
			require.Equal(stats.CPUStats.ThrottlingData.ThrottledTime, ru.ResourceUsage.CpuStats.ThrottledTime)
		} else {
			require.Equal(stats.MemoryStats.PrivateWorkingSet, ru.ResourceUsage.MemoryStats.RSS)
			require.Equal(stats.MemoryStats.Commit, ru.ResourceUsage.MemoryStats.Usage)
			require.Equal(stats.MemoryStats.CommitPeak, ru.ResourceUsage.MemoryStats.MaxUsage)
			require.Equal(stats.CPUStats.ThrottlingData.ThrottledPeriods, ru.ResourceUsage.CpuStats.ThrottledPeriods)
			require.Equal(stats.CPUStats.ThrottlingData.ThrottledTime, ru.ResourceUsage.CpuStats.ThrottledTime)

		}
	case <-time.After(time.Second):
		require.Fail("receiving stats should not block here")
	}
}
