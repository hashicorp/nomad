package taskrunner

import (
	"context"
	"testing"
	"time"

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
	// err is returned by Stats if it is non-nil
	err error
}

func (m *mockDriverStats) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
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

// TestTaskRunner_StatsHook_PoststartExited asserts the stats hook starts and
// stops.
func TestTaskRunner_StatsHook_PoststartExited(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	select {
	case ru := <-su.Ch:
		t.Fatalf("unexpected update after exit (firstrun=%v; update=%v", firstrun, ru.Timestamp)
	case <-time.After(2 * interval):
		// Ok! No update after exit as expected.
	}
}

// TestTaskRunner_StatsHook_NotImplemented asserts the stats hook stops if the
// driver returns NotImplemented.
func TestTaskRunner_StatsHook_NotImplemented(t *testing.T) {
	t.Parallel()

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
