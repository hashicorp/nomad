// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"runtime"
	"sync"
	"testing"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
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
