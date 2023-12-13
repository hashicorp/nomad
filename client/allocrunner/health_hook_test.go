// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"sync"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
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
var _ interfaces.RunnerPostrunHook = (*allocHealthWatcherHook)(nil)
var _ interfaces.ShutdownHook = (*allocHealthWatcherHook)(nil)

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

func (m *mockHealthSetter) HasHealth() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthy != nil
}

// TestHealthHook_PrerunPostrun asserts a health hook does not error if it is
// run and postrunned.
func TestHealthHook_PrerunPostrun(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	logger := testlog.HCLogger(t)

	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	consul := regMock.NewServiceRegistrationHandler(logger)
	hs := &mockHealthSetter{}

	checks := new(mock.CheckShim)
	alloc := mock.Alloc()
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks)

	// Assert we implemented the right interfaces
	prerunh, ok := h.(interfaces.RunnerPrerunHook)
	require.True(ok)
	_, ok = h.(interfaces.RunnerUpdateHook)
	require.True(ok)
	postrunh, ok := h.(interfaces.RunnerPostrunHook)
	require.True(ok)

	// Prerun
	require.NoError(prerunh.Prerun())

	// Assert isDeploy is false (other tests peek at isDeploy to determine
	// if an Update applied)
	ahw := h.(*allocHealthWatcherHook)
	ahw.hookLock.Lock()
	assert.False(t, ahw.isDeploy)
	ahw.hookLock.Unlock()

	// Postrun
	require.NoError(postrunh.Postrun())
}

// TestHealthHook_PrerunUpdatePostrun asserts Updates may be applied concurrently.
func TestHealthHook_PrerunUpdatePostrun(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.Alloc()

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	consul := regMock.NewServiceRegistrationHandler(logger)
	hs := &mockHealthSetter{}

	checks := new(mock.CheckShim)
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks).(*allocHealthWatcherHook)

	// Prerun
	require.NoError(h.Prerun())

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

	// Postrun
	require.NoError(h.Postrun())
}

// TestHealthHook_UpdatePrerunPostrun asserts that a hook may have Update
// called before Prerun.
func TestHealthHook_UpdatePrerunPostrun(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.Alloc()

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	consul := regMock.NewServiceRegistrationHandler(logger)
	hs := &mockHealthSetter{}

	checks := new(mock.CheckShim)
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks).(*allocHealthWatcherHook)

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
	require.NoError(h.Prerun())

	// Assert that the Update took affect by isDeploy being true
	h.hookLock.Lock()
	assert.True(t, h.isDeploy)
	h.hookLock.Unlock()

	// Postrun
	require.NoError(h.Postrun())
}

// TestHealthHook_Postrun asserts that a hook may have only Postrun called.
func TestHealthHook_Postrun(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	consul := regMock.NewServiceRegistrationHandler(logger)
	hs := &mockHealthSetter{}

	alloc := mock.Alloc()
	checks := new(mock.CheckShim)
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks).(*allocHealthWatcherHook)

	// Postrun
	require.NoError(h.Postrun())
}

// TestHealthHook_SetHealth_healthy asserts SetHealth is called when health status is
// set. Uses task state and health checks.
func TestHealthHook_SetHealth_healthy(t *testing.T) {
	ci.Parallel(t)
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
	taskRegs := map[string]*serviceregistration.ServiceRegistrations{
		task.Name: {
			Services: map[string]*serviceregistration.ServiceRegistration{
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

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	// Don't reply on the first call
	called := false
	consul := regMock.NewServiceRegistrationHandler(logger)
	consul.AllocRegistrationsFn = func(string) (*serviceregistration.AllocRegistration, error) {
		if !called {
			called = true
			return nil, nil
		}

		reg := &serviceregistration.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	hs := newMockHealthSetter()

	checks := new(mock.CheckShim)
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks).(*allocHealthWatcherHook)

	// Prerun
	require.NoError(h.Prerun())

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

	// Postrun
	require.NoError(h.Postrun())
}

// TestHealthHook_SetHealth_unhealthy asserts SetHealth notices unhealthy allocs
func TestHealthHook_SetHealth_unhealthy(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Migrate.MinHealthyTime = 1 // let's speed things up
	task := alloc.Job.TaskGroups[0].Tasks[0]

	newCheck := task.Services[0].Checks[0].Copy()
	newCheck.Name = "failing-check"
	task.Services[0].Checks = append(task.Services[0].Checks, newCheck)

	// Synthesize running alloc and tasks
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		task.Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
	}

	// Make Consul response
	checkHealthy := &consulapi.AgentCheck{
		Name:   task.Services[0].Checks[0].Name,
		Status: consulapi.HealthPassing,
	}
	checksUnhealthy := &consulapi.AgentCheck{
		Name:   task.Services[0].Checks[1].Name,
		Status: consulapi.HealthCritical,
	}
	taskRegs := map[string]*serviceregistration.ServiceRegistrations{
		task.Name: {
			Services: map[string]*serviceregistration.ServiceRegistration{
				task.Services[0].Name: {
					Service: &consulapi.AgentService{
						ID:      "foo",
						Service: task.Services[0].Name,
					},
					Checks: []*consulapi.AgentCheck{checkHealthy, checksUnhealthy},
				},
			},
		},
	}

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	// Don't reply on the first call
	called := false
	consul := regMock.NewServiceRegistrationHandler(logger)
	consul.AllocRegistrationsFn = func(string) (*serviceregistration.AllocRegistration, error) {
		if !called {
			called = true
			return nil, nil
		}

		reg := &serviceregistration.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	hs := newMockHealthSetter()

	checks := new(mock.CheckShim)
	h := newAllocHealthWatcherHook(logger, alloc.Copy(), taskEnvBuilderFactory(alloc), hs, b.Listen(), consul, checks).(*allocHealthWatcherHook)

	// Prerun
	require.NoError(h.Prerun())

	// Wait to ensure we don't get a healthy status
	select {
	case <-time.After(2 * time.Second):
		// great no healthy status
	case health := <-hs.healthCh:
		require.Fail("expected no health event", "got %v", health)
	}

	// Postrun
	require.NoError(h.Postrun())
}

// TestHealthHook_SystemNoop asserts that system jobs return the noop tracker.
func TestHealthHook_SystemNoop(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.SystemAlloc()
	h := newAllocHealthWatcherHook(testlog.HCLogger(t), alloc.Copy(), taskEnvBuilderFactory(alloc), nil, nil, nil, nil)

	// Assert that it's the noop impl
	_, ok := h.(noopAllocHealthWatcherHook)
	require.True(t, ok)

	// Assert the noop impl does not implement any hooks
	_, ok = h.(interfaces.RunnerPrerunHook)
	require.False(t, ok)
	_, ok = h.(interfaces.RunnerUpdateHook)
	require.False(t, ok)
	_, ok = h.(interfaces.RunnerPostrunHook)
	require.False(t, ok)
	_, ok = h.(interfaces.ShutdownHook)
	require.False(t, ok)
}

// TestHealthHook_BatchNoop asserts that batch jobs return the noop tracker.
func TestHealthHook_BatchNoop(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	h := newAllocHealthWatcherHook(testlog.HCLogger(t), alloc.Copy(), taskEnvBuilderFactory(alloc), nil, nil, nil, nil)

	// Assert that it's the noop impl
	_, ok := h.(noopAllocHealthWatcherHook)
	require.True(t, ok)
}

func taskEnvBuilderFactory(alloc *structs.Allocation) func() *taskenv.Builder {
	return func() *taskenv.Builder {
		return taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region)
	}
}
