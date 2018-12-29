package allocrunner

import (
	"context"
	"sync"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statically assert health hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*allocHealthWatcherHook)(nil)
var _ interfaces.RunnerUpdateHook = (*allocHealthWatcherHook)(nil)
var _ interfaces.RunnerDestroyHook = (*allocHealthWatcherHook)(nil)

// allocHealth is emitted to a chan whenever SetHealth is called
type allocHealth struct {
	healthy    bool
	taskEvents map[string]*structs.TaskEvent
}

// mockHealthSetter implements healthSetter that stores health internally
type mockHealthSetter struct {
	setCalls   int
	clearCalls int
	healthy    *bool
	isDeploy   *bool
	taskEvents map[string]*structs.TaskEvent
	mu         sync.Mutex

	healthCh chan allocHealth
}

// newMockHealthSetter returns a mock HealthSetter that emits all SetHealth
// calls on a buffered chan. Callers who do need need notifications of health
// changes may just create the struct directly.
func newMockHealthSetter() *mockHealthSetter {
	return &mockHealthSetter{
		healthCh: make(chan allocHealth, 1),
	}
}

func (m *mockHealthSetter) SetHealth(healthy, isDeploy bool, taskEvents map[string]*structs.TaskEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setCalls++
	m.healthy = &healthy
	m.isDeploy = &isDeploy
	m.taskEvents = taskEvents

	if m.healthCh != nil {
		m.healthCh <- allocHealth{healthy, taskEvents}
	}
}

func (m *mockHealthSetter) ClearHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clearCalls++
	m.healthy = nil
	m.isDeploy = nil
	m.taskEvents = nil
}

// TestHealthHook_PrerunDestroy asserts a health hook does not error if it is run and destroyed.
func TestHealthHook_PrerunDestroy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	logger := testlog.HCLogger(t)

	consul := consul.NewMockConsulServiceClient(t, logger)
	hs := &mockHealthSetter{}

	h := newAllocHealthWatcherHook(logger, mock.Alloc(), hs, b.Listen(), consul)

	// Assert we implemented the right interfaces
	prerunh, ok := h.(interfaces.RunnerPrerunHook)
	require.True(ok)
	_, ok = h.(interfaces.RunnerUpdateHook)
	require.True(ok)
	destroyh, ok := h.(interfaces.RunnerDestroyHook)
	require.True(ok)

	// Prerun
	require.NoError(prerunh.Prerun(context.Background()))

	// Assert isDeploy is false (other tests peek at isDeploy to determine
	// if an Update applied)
	ahw := h.(*allocHealthWatcherHook)
	ahw.hookLock.Lock()
	assert.False(t, ahw.isDeploy)
	ahw.hookLock.Unlock()

	// Destroy
	require.NoError(destroyh.Destroy())
}

// TestHealthHook_PrerunUpdateDestroy asserts Updates may be applied concurrently.
func TestHealthHook_PrerunUpdateDestroy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.Alloc()

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	logger := testlog.HCLogger(t)
	consul := consul.NewMockConsulServiceClient(t, logger)
	hs := &mockHealthSetter{}

	h := newAllocHealthWatcherHook(logger, alloc.Copy(), hs, b.Listen(), consul).(*allocHealthWatcherHook)

	// Prerun
	require.NoError(h.Prerun(context.Background()))

	// Update multiple times in a goroutine to mimic Client behavior
	// (Updates are concurrent with alloc runner but are applied serially).
	errs := make(chan error, 2)
	go func() {
		defer close(errs)
		for i := 0; i < cap(errs); i++ {
			alloc.AllocModifyIndex++
			errs <- h.Update(&interfaces.RunnerUpdateRequest{Alloc: alloc.Copy()})
		}
	}()

	for err := range errs {
		assert.NoError(t, err)
	}

	// Destroy
	require.NoError(h.Destroy())
}

// TestHealthHook_UpdatePrerunDestroy asserts that a hook may have Update
// called before Prerun.
func TestHealthHook_UpdatePrerunDestroy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.Alloc()

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	logger := testlog.HCLogger(t)
	consul := consul.NewMockConsulServiceClient(t, logger)
	hs := &mockHealthSetter{}

	h := newAllocHealthWatcherHook(logger, alloc.Copy(), hs, b.Listen(), consul).(*allocHealthWatcherHook)

	// Set a DeploymentID to cause ClearHealth to be called
	alloc.DeploymentID = uuid.Generate()

	// Update in a goroutine to mimic Client behavior (Updates are
	// concurrent with alloc runner).
	errs := make(chan error, 1)
	go func(alloc *structs.Allocation) {
		errs <- h.Update(&interfaces.RunnerUpdateRequest{Alloc: alloc})
		close(errs)
	}(alloc.Copy())

	for err := range errs {
		assert.NoError(t, err)
	}

	// Prerun should be a noop
	require.NoError(h.Prerun(context.Background()))

	// Assert that the Update took affect by isDeploy being true
	h.hookLock.Lock()
	assert.True(t, h.isDeploy)
	h.hookLock.Unlock()

	// Destroy
	require.NoError(h.Destroy())
}

// TestHealthHook_Destroy asserts that a hook may have only Destroy called.
func TestHealthHook_Destroy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	logger := testlog.HCLogger(t)
	consul := consul.NewMockConsulServiceClient(t, logger)
	hs := &mockHealthSetter{}

	h := newAllocHealthWatcherHook(logger, mock.Alloc(), hs, b.Listen(), consul).(*allocHealthWatcherHook)

	// Destroy
	require.NoError(h.Destroy())
}

// TestHealthHook_SetHealth asserts SetHealth is called when health status is
// set. Uses task state and health checks.
func TestHealthHook_SetHealth(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Migrate.MinHealthyTime = 1 // let's speed things up
	task := alloc.Job.TaskGroups[0].Tasks[0]

	// Synthesize running alloc and tasks
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		task.Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
	}

	// Make Consul response
	check := &consulapi.AgentCheck{
		Name:   task.Services[0].Checks[0].Name,
		Status: consulapi.HealthPassing,
	}
	taskRegs := map[string]*agentconsul.TaskRegistration{
		task.Name: {
			Services: map[string]*agentconsul.ServiceRegistration{
				task.Services[0].Name: {
					Service: &consulapi.AgentService{
						ID:      "foo",
						Service: task.Services[0].Name,
					},
					Checks: []*consulapi.AgentCheck{check},
				},
			},
		},
	}

	b := cstructs.NewAllocBroadcaster()
	defer b.Close()

	logger := testlog.HCLogger(t)

	// Don't reply on the first call
	called := false
	consul := consul.NewMockConsulServiceClient(t, logger)
	consul.AllocRegistrationsFn = func(string) (*agentconsul.AllocRegistration, error) {
		if !called {
			called = true
			return nil, nil
		}

		reg := &agentconsul.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	hs := newMockHealthSetter()

	h := newAllocHealthWatcherHook(logger, alloc.Copy(), hs, b.Listen(), consul).(*allocHealthWatcherHook)

	// Prerun
	require.NoError(h.Prerun(context.Background()))

	// Wait for health to be set (healthy)
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for health to be set")
	case health := <-hs.healthCh:
		require.True(health.healthy)

		// Healthy allocs shouldn't emit task events
		ev := health.taskEvents[task.Name]
		require.Nilf(ev, "%#v", health.taskEvents)
	}

	// Destroy
	require.NoError(h.Destroy())
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
