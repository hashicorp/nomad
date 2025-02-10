// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/drivers/docker/util"
	"github.com/shoenig/test/must"
)

func TestDriver_DockerStatsCollector(t *testing.T) {
	ci.Parallel(t)

	stats := &containerapi.Stats{}
	stats.CPUStats.ThrottlingData.Periods = 10
	stats.CPUStats.ThrottlingData.ThrottledPeriods = 10
	stats.CPUStats.ThrottlingData.ThrottledTime = 10

	stats.MemoryStats.Stats = map[string]uint64{}
	stats.MemoryStats.Stats["file_mapped"] = 1024
	stats.MemoryStats.Usage = 5651904
	stats.MemoryStats.MaxUsage = 6651904
	stats.MemoryStats.Commit = 123231
	stats.MemoryStats.CommitPeak = 321323
	stats.MemoryStats.PrivateWorkingSet = 62222

	ru := util.DockerStatsToTaskResourceUsage(stats, cpustats.Compute{})

	if runtime.GOOS != "windows" {
		must.Eq(t, stats.MemoryStats.Stats["file_mapped"], ru.ResourceUsage.MemoryStats.MappedFile)
		must.Eq(t, stats.MemoryStats.Usage, ru.ResourceUsage.MemoryStats.Usage)
		must.Eq(t, stats.MemoryStats.MaxUsage, ru.ResourceUsage.MemoryStats.MaxUsage)
		must.Eq(t, stats.CPUStats.ThrottlingData.ThrottledPeriods, ru.ResourceUsage.CpuStats.ThrottledPeriods)
		must.Eq(t, stats.CPUStats.ThrottlingData.ThrottledTime, ru.ResourceUsage.CpuStats.ThrottledTime)
	} else {
		must.Eq(t, stats.MemoryStats.PrivateWorkingSet, ru.ResourceUsage.MemoryStats.RSS)
		must.Eq(t, stats.MemoryStats.Commit, ru.ResourceUsage.MemoryStats.Usage)
		must.Eq(t, stats.MemoryStats.CommitPeak, ru.ResourceUsage.MemoryStats.MaxUsage)
		must.Eq(t, stats.CPUStats.ThrottlingData.ThrottledPeriods, ru.ResourceUsage.CpuStats.ThrottledPeriods)
		must.Eq(t, stats.CPUStats.ThrottlingData.ThrottledTime, ru.ResourceUsage.CpuStats.ThrottledTime)

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
	must.True(t, closed)

	select {
	case _, ok := <-recvCh:
		must.False(t, ok)
	default:
		t.Fatal("expect recvCh to be closed")
	}

	// Assert sending and closing never fails
	destCh.send(res)
	destCh.close()
	destCh.close()
	destCh.send(res)
}

func Test_taskHandle_collectDockerStats(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	// Start a Docker container and wait for it to be running, so we can
	// guarantee stats generation.
	driverCfg, dockerTaskConfig, _ := dockerTask(t)

	must.NoError(t, driverCfg.EncodeConcreteDriverConfig(dockerTaskConfig))

	_, driverHarness, handle, cleanup := dockerSetup(t, driverCfg, nil)
	defer cleanup()
	must.NoError(t, driverHarness.WaitUntilStarted(driverCfg.ID, 5*time.Second))

	// Generate a context, so the test doesn't hang on Docker problems and
	// execute a single collection of the stats.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dockerStats, err := handle.collectDockerStats(ctx)
	must.NoError(t, err)
	must.NotNil(t, dockerStats)

	// Ensure all the stats we use for calculating CPU percentages within
	// DockerStatsToTaskResourceUsage are present and non-zero.
	must.NonZero(t, dockerStats.CPUStats.CPUUsage.TotalUsage)
	must.NonZero(t, dockerStats.CPUStats.CPUUsage.TotalUsage)

	must.NonZero(t, dockerStats.PreCPUStats.CPUUsage.TotalUsage)
	must.NonZero(t, dockerStats.PreCPUStats.CPUUsage.TotalUsage)

	// System usage is only populated on Linux machines. GitHub Actions Windows
	// runners do not have UsageInKernelmode or UsageInUsermode populated and
	// these datapoints are not used by the Windows stats usage function. Also
	// wrap the Linux specific memory stats.
	if runtime.GOOS == "linux" {
		must.NonZero(t, dockerStats.CPUStats.SystemUsage)
		must.NonZero(t, dockerStats.CPUStats.CPUUsage.UsageInKernelmode)
		must.NonZero(t, dockerStats.CPUStats.CPUUsage.UsageInUsermode)

		must.NonZero(t, dockerStats.PreCPUStats.SystemUsage)
		must.NonZero(t, dockerStats.PreCPUStats.CPUUsage.UsageInKernelmode)
		must.NonZero(t, dockerStats.PreCPUStats.CPUUsage.UsageInUsermode)

		must.NonZero(t, dockerStats.MemoryStats.Usage)
		must.MapContainsKey(t, dockerStats.MemoryStats.Stats, "file_mapped")
	}

	// Test Windows specific memory stats are collected as and when expected.
	if runtime.GOOS == "windows" {
		must.NonZero(t, dockerStats.MemoryStats.PrivateWorkingSet)
		must.NonZero(t, dockerStats.MemoryStats.Commit)
		must.NonZero(t, dockerStats.MemoryStats.CommitPeak)
	}
}
