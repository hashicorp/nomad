// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPoststartHook = (*statsHook)(nil)
var _ interfaces.TaskExitedHook = (*statsHook)(nil)
var _ interfaces.ShutdownHook = (*statsHook)(nil)

type mockStatsUpdater struct {
	// Ch is sent task resource usage updates if not nil
	Ch chan *cstructs.TaskResourceUsage
}

// newMockStatsUpdater returns a mockStatsUpdater that blocks on Ch for every
// call to UpdateStats
func newMockStatsUpdater() *mockStatsUpdater {
	return &mockStatsUpdater{
		Ch: make(chan *cstructs.TaskResourceUsage),
	}
}

func (m *mockStatsUpdater) UpdateStats(ru *cstructs.TaskResourceUsage) {
	if m.Ch != nil {
		m.Ch <- ru
	}
}

type mockDriverStats struct {
	called uint32

	// err is returned by Stats if it is non-nil
	err error
}

func (m *mockDriverStats) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	atomic.AddUint32(&m.called, 1)

	if m.err != nil {
		return nil, m.err
	}
	ru := &cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: &cstructs.MemoryStats{
				RSS:      1,
				Measured: []string{"RSS"},
			},
			CpuStats: &cstructs.CpuStats{
				SystemMode: 1,
				Measured:   []string{"System Mode"},
			},
		},
		Timestamp: time.Now().UnixNano(),
		Pids:      map[string]*cstructs.ResourceUsage{},
	}
	ru.Pids["task"] = ru.ResourceUsage
	ch := make(chan *cstructs.TaskResourceUsage)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		case ch <- ru:
		}
	}()
	return ch, nil
}

func (m *mockDriverStats) Called() int {
	return int(atomic.LoadUint32(&m.called))
}

// TestTaskRunner_StatsHook_PoststartExited asserts the stats hook starts and
// stops.
func TestTaskRunner_StatsHook_PoststartExited(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	logger := testlog.HCLogger(t)
	su := newMockStatsUpdater()
	ds := new(mockDriverStats)

	poststartReq := &interfaces.TaskPoststartRequest{DriverStats: ds}

	// Create hook
	h := newStatsHook(su, time.Minute, logger)

	// Always call Exited to cleanup goroutines
	defer h.Exited(context.Background(), nil, nil)

	// Run prestart
	require.NoError(h.Poststart(context.Background(), poststartReq, nil))

	// An initial stats collection should run and call the updater
	select {
	case ru := <-su.Ch:
		require.Equal(uint64(1), ru.ResourceUsage.MemoryStats.RSS)
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for initial stats collection")
	}

	require.NoError(h.Exited(context.Background(), nil, nil))
}

// TestTaskRunner_StatsHook_Periodic asserts the stats hook collects stats on
// an interval.
func TestTaskRunner_StatsHook_Periodic(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	logger := testlog.HCLogger(t)
	su := newMockStatsUpdater()

	ds := new(mockDriverStats)
	poststartReq := &interfaces.TaskPoststartRequest{DriverStats: ds}

	// interval needs to be high enough that even on a slow/busy VM
	// Exited() can complete within the interval.
	const interval = 500 * time.Millisecond

	h := newStatsHook(su, interval, logger)
	defer h.Exited(context.Background(), nil, nil)

	// Run prestart
	require.NoError(h.Poststart(context.Background(), poststartReq, nil))

	// An initial stats collection should run and call the updater
	var firstrun int64
	select {
	case ru := <-su.Ch:
		if ru.Timestamp <= 0 {
			t.Fatalf("expected nonzero timestamp (%v)", ru.Timestamp)
		}
		firstrun = ru.Timestamp
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for initial stats collection")
	}

	// Should get another update in ~500ms (see interval above)
	select {
	case ru := <-su.Ch:
		if ru.Timestamp <= firstrun {
			t.Fatalf("expected timestamp (%v) after first run (%v)", ru.Timestamp, firstrun)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for second stats collection")
	}

	// Exiting should prevent further updates
	require.NoError(h.Exited(context.Background(), nil, nil))

	// Should *not* get another update in ~500ms (see interval above)
	// we may get a single update due to race with exit
	timeout := time.After(2 * interval)
	firstUpdate := true

WAITING:
	select {
	case ru := <-su.Ch:
		if firstUpdate {
			firstUpdate = false
			goto WAITING
		}
		t.Fatalf("unexpected update after exit (firstrun=%v; update=%v", firstrun, ru.Timestamp)
	case <-timeout:
		// Ok! No update after exit as expected.
	}
}

// TestTaskRunner_StatsHook_NotImplemented asserts the stats hook stops if the
// driver returns NotImplemented.
func TestTaskRunner_StatsHook_NotImplemented(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	logger := testlog.HCLogger(t)
	su := newMockStatsUpdater()
	ds := &mockDriverStats{
		err: cstructs.DriverStatsNotImplemented,
	}

	poststartReq := &interfaces.TaskPoststartRequest{DriverStats: ds}

	h := newStatsHook(su, 1, logger)
	defer h.Exited(context.Background(), nil, nil)

	// Run prestart
	require.NoError(h.Poststart(context.Background(), poststartReq, nil))

	// An initial stats collection should run and *not* call the updater
	select {
	case ru := <-su.Ch:
		t.Fatalf("unexpected resource update (timestamp=%v)", ru.Timestamp)
	case <-time.After(500 * time.Millisecond):
		// Ok! No update received because error was returned
	}
}

// TestTaskRunner_StatsHook_Backoff asserts that stats hook does some backoff
// even if the driver doesn't support intervals well
func TestTaskRunner_StatsHook_Backoff(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	su := newMockStatsUpdater()
	ds := &mockDriverStats{}

	poststartReq := &interfaces.TaskPoststartRequest{DriverStats: ds}

	h := newStatsHook(su, time.Minute, logger)
	defer h.Exited(context.Background(), nil, nil)

	// Run prestart
	require.NoError(t, h.Poststart(context.Background(), poststartReq, nil))

	timeout := time.After(500 * time.Millisecond)

DRAIN:
	for {
		select {
		case <-su.Ch:
		case <-timeout:
			break DRAIN
		}
	}

	require.Equal(t, ds.Called(), 1)
}
