// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"runtime"
	"sync"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/stretchr/testify/require"
)

func TestDriver_DockerStatsCollector(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	src := make(chan *docker.Stats)
	defer close(src)
	dst, recvCh := newStatsChanPipe()
	defer dst.close()
	stats := &docker.Stats{}
	stats.CPUStats.ThrottlingData.Periods = 10
	stats.CPUStats.ThrottlingData.ThrottledPeriods = 10
	stats.CPUStats.ThrottlingData.ThrottledTime = 10

	stats.MemoryStats.Stats.Rss = 6537216
	stats.MemoryStats.Stats.Cache = 1234
	stats.MemoryStats.Stats.Swap = 0
	stats.MemoryStats.Stats.MappedFile = 1024
	stats.MemoryStats.Usage = 5651904
	stats.MemoryStats.MaxUsage = 6651904
	stats.MemoryStats.Commit = 123231
	stats.MemoryStats.CommitPeak = 321323
	stats.MemoryStats.PrivateWorkingSet = 62222

	go dockerStatsCollector(dst, src, time.Second, top)

	select {
	case src <- stats:
	case <-time.After(time.Second):
		require.Fail("sending stats should not block here")
	}

	select {
	case ru := <-recvCh:
		if runtime.GOOS != "windows" {
			require.Equal(stats.MemoryStats.Stats.Rss, ru.ResourceUsage.MemoryStats.RSS)
			require.Equal(stats.MemoryStats.Stats.Cache, ru.ResourceUsage.MemoryStats.Cache)
			require.Equal(stats.MemoryStats.Stats.Swap, ru.ResourceUsage.MemoryStats.Swap)
			require.Equal(stats.MemoryStats.Stats.MappedFile, ru.ResourceUsage.MemoryStats.MappedFile)
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

// TestDriver_DockerUsageSender asserts that the TaskResourceUsage chan wrapper
// supports closing and sending on a chan from concurrent goroutines.
func TestDriver_DockerUsageSender(t *testing.T) {
	ci.Parallel(t)

	// sample payload
	res := &cstructs.TaskResourceUsage{}

	destCh, recvCh := newStatsChanPipe()

	// Sending should never fail
	destCh.send(res)
	destCh.send(res)
	destCh.send(res)

	// Clear chan
	<-recvCh

	// Send and close concurrently to let the race detector help us out
	wg := sync.WaitGroup{}
	wg.Add(3)

	// Sender
	go func() {
		destCh.send(res)
		wg.Done()
	}()

	// Closer
	go func() {
		destCh.close()
		wg.Done()
	}()

	// Clear recv chan
	go func() {
		for range recvCh {
		}
		wg.Done()
	}()

	wg.Wait()

	// Assert closed
	destCh.mu.Lock()
	closed := destCh.closed
	destCh.mu.Unlock()
	require.True(t, closed)

	select {
	case _, ok := <-recvCh:
		require.False(t, ok)
	default:
		require.Fail(t, "expect recvCh to be closed")
	}

	// Assert sending and closing never fails
	destCh.send(res)
	destCh.close()
	destCh.close()
	destCh.send(res)
}
