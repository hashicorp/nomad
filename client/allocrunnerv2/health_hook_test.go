package allocrunnerv2

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// statically assert health hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*allocHealthWatcherHook)(nil)
var _ interfaces.RunnerUpdateHook = (*allocHealthWatcherHook)(nil)
var _ interfaces.RunnerDestroyHook = (*allocHealthWatcherHook)(nil)

// mockHealthSetter implements healthSetter that stores health internally
type mockHealthSetter struct {
	setCalls   int
	clearCalls int
	healthy    *bool
	isDeploy   *bool
	taskEvents map[string]*structs.TaskEvent
	mu         sync.Mutex
}

func (m *mockHealthSetter) SetHealth(healthy, isDeploy bool, taskEvents map[string]*structs.TaskEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setCalls++
	m.healthy = &healthy
	m.isDeploy = &isDeploy
	m.taskEvents = taskEvents
}

func (m *mockHealthSetter) ClearHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clearCalls++
	m.healthy = nil
	m.isDeploy = nil
	m.taskEvents = nil
}

func TestHealthHook_PrerunDestroy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	consul := consul.NewMockConsulServiceClient(t)
	hs := &mockHealthSetter{}

	h := newAllocHealthWatcherHook(testlog.HCLogger(t), mock.Alloc(), hs, b.Listen(), consul)

	// Assert we implemented the right interfaces
	prerunh, ok := h.(interfaces.RunnerPrerunHook)
	require.True(ok)
	_, ok = h.(interfaces.RunnerUpdateHook)
	require.True(ok)
	destroyh, ok := h.(interfaces.RunnerDestroyHook)
	require.True(ok)

	// Prerun
	require.NoError(prerunh.Prerun(context.Background()))

	// Destroy
	require.NoError(destroyh.Destroy())
}

// TestHealthHook_SystemNoop asserts that system jobs return the noop tracker.
func TestHealthHook_SystemNoop(t *testing.T) {
	t.Parallel()

	h := newAllocHealthWatcherHook(testlog.HCLogger(t), mock.SystemAlloc(), nil, nil, nil)

	// Assert that it's the noop impl
	_, ok := h.(noopAllocHealthWatcherHook)
	require.True(t, ok)

	// Assert the noop impl does not implement any hooks
	_, ok = h.(interfaces.RunnerPrerunHook)
	require.False(t, ok)
	_, ok = h.(interfaces.RunnerUpdateHook)
	require.False(t, ok)
	_, ok = h.(interfaces.RunnerDestroyHook)
	require.False(t, ok)
}

// TestHealthHook_BatchNoop asserts that batch jobs return the noop tracker.
func TestHealthHook_BatchNoop(t *testing.T) {
	t.Parallel()

	h := newAllocHealthWatcherHook(testlog.HCLogger(t), mock.BatchAlloc(), nil, nil, nil)

	// Assert that it's the noop impl
	_, ok := h.(noopAllocHealthWatcherHook)
	require.True(t, ok)
}

// TestHealthHook_WatchSema asserts only one caller can acquire the watchSema
// lock at once and all other callers are blocked.
func TestHealthHook_WatchSema(t *testing.T) {
	t.Parallel()

	s := newWatchSema()

	s.acquire()

	// Second acquire should block
	acq2 := make(chan struct{})
	go func() {
		s.acquire()
		close(acq2)
	}()

	select {
	case <-acq2:
		t.Fatalf("should not have been able to acquire lock")
	case <-time.After(33 * time.Millisecond):
		// Ok! It's blocking!
	}

	// Release and assert second acquire is now active
	s.release()

	select {
	case <-acq2:
		// Ok! Second acquire unblocked!
	case <-time.After(33 * time.Millisecond):
		t.Fatalf("second acquire should not have blocked")
	}

	// wait() should block until lock is released
	waitCh := make(chan struct{})
	go func() {
		s.wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		t.Fatalf("should not have been able to acquire lock")
	case <-time.After(33 * time.Millisecond):
		// Ok! It's blocking!
	}

	// Release and assert wait() finished
	s.release()

	select {
	case <-waitCh:
		// Ok! wait() unblocked!
	case <-time.After(33 * time.Millisecond):
		t.Fatalf("wait should not have blocked")
	}

	// Assert wait() did not acquire the lock
	s.acquire()
	s.release()
}
