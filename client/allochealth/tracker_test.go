package allochealth

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestTracker_Checks_Healthy(t *testing.T) {
	t.Parallel()

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
	taskRegs := map[string]*agentconsul.ServiceRegistrations{
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

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	// Don't reply on the first call
	var called uint64
	consul := consul.NewMockConsulServiceClient(t, logger)
	consul.AllocRegistrationsFn = func(string) (*agentconsul.AllocRegistration, error) {
		if atomic.AddUint64(&called, 1) == 1 {
			return nil, nil
		}

		reg := &agentconsul.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	checkInterval := 10 * time.Millisecond
	tracker := NewTracker(ctx, logger, alloc, b.Listen(), consul,
		time.Millisecond, true)
	tracker.checkLookupInterval = checkInterval
	tracker.Start()

	select {
	case <-time.After(4 * checkInterval):
		require.Fail(t, "timed out while waiting for health")
	case h := <-tracker.HealthyCh():
		require.True(t, h)
	}
}

func TestTracker_Checks_Unhealthy(t *testing.T) {
	t.Parallel()

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
	taskRegs := map[string]*agentconsul.ServiceRegistrations{
		task.Name: {
			Services: map[string]*agentconsul.ServiceRegistration{
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
	var called uint64
	consul := consul.NewMockConsulServiceClient(t, logger)
	consul.AllocRegistrationsFn = func(string) (*agentconsul.AllocRegistration, error) {
		if atomic.AddUint64(&called, 1) == 1 {
			return nil, nil
		}

		reg := &agentconsul.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	checkInterval := 10 * time.Millisecond
	tracker := NewTracker(ctx, logger, alloc, b.Listen(), consul,
		time.Millisecond, true)
	tracker.checkLookupInterval = checkInterval
	tracker.Start()

	testutil.WaitForResult(func() (bool, error) {
		lookup := atomic.LoadUint64(&called)
		return lookup < 4, fmt.Errorf("wait to get more task registration lookups: %v", lookup)
	}, func(err error) {
		require.NoError(t, err)
	})

	tracker.l.Lock()
	require.False(t, tracker.checksHealthy)
	tracker.l.Unlock()

	select {
	case v := <-tracker.HealthyCh():
		require.Failf(t, "expected no health value", " got %v", v)
	default:
		// good
	}
}

func TestTracker_Healthy_IfBothTasksAndConsulChecksAreHealthy(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	tracker := NewTracker(ctx, logger, alloc, nil, nil,
		time.Millisecond, true)

	assertNoHealth := func() {
		require.NoError(t, tracker.ctx.Err())
		select {
		case v := <-tracker.HealthyCh():
			require.Failf(t, "unexpected healthy event", "got %v", v)
		default:
		}
	}

	// first set task health without checks
	tracker.setTaskHealth(true, false)
	assertNoHealth()

	// now fail task health again before checks are successful
	tracker.setTaskHealth(false, false)
	assertNoHealth()

	// now pass health checks - do not propagate health yet
	tracker.setCheckHealth(true)
	assertNoHealth()

	// set tasks to healthy - don't propagate health yet, wait for the next check
	tracker.setTaskHealth(true, false)
	assertNoHealth()

	// set checks to true, now propagate health status
	tracker.setCheckHealth(true)

	require.Error(t, tracker.ctx.Err())
	select {
	case v := <-tracker.HealthyCh():
		require.True(t, v)
	default:
		require.Fail(t, "expected a health status")
	}
}

// TestTracker_Checks_Healthy_Before_TaskHealth asserts that we mark an alloc
// healthy, if the checks pass before task health pass
func TestTracker_Checks_Healthy_Before_TaskHealth(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Migrate.MinHealthyTime = 1 // let's speed things up
	task := alloc.Job.TaskGroups[0].Tasks[0]

	// new task starting unhealthy, without services
	task2 := task.Copy()
	task2.Name = task2.Name + "2"
	task2.Services = nil
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)

	// Synthesize running alloc and tasks
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		task.Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
		task2.Name: {
			State: structs.TaskStatePending,
		},
	}

	// Make Consul response
	check := &consulapi.AgentCheck{
		Name:   task.Services[0].Checks[0].Name,
		Status: consulapi.HealthPassing,
	}
	taskRegs := map[string]*agentconsul.ServiceRegistrations{
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

	logger := testlog.HCLogger(t)
	b := cstructs.NewAllocBroadcaster(logger)
	defer b.Close()

	// Don't reply on the first call
	var called uint64
	consul := consul.NewMockConsulServiceClient(t, logger)
	consul.AllocRegistrationsFn = func(string) (*agentconsul.AllocRegistration, error) {
		if atomic.AddUint64(&called, 1) == 1 {
			return nil, nil
		}

		reg := &agentconsul.AllocRegistration{
			Tasks: taskRegs,
		}

		return reg, nil
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	checkInterval := 10 * time.Millisecond
	tracker := NewTracker(ctx, logger, alloc, b.Listen(), consul,
		time.Millisecond, true)
	tracker.checkLookupInterval = checkInterval
	tracker.Start()

	// assert that we don't get marked healthy
	select {
	case <-time.After(4 * checkInterval):
		// still unhealthy, good
	case h := <-tracker.HealthyCh():
		require.Fail(t, "unexpected health event", h)
	}
	require.False(t, tracker.tasksHealthy)
	require.False(t, tracker.checksHealthy)

	// now set task to healthy
	runningAlloc := alloc.Copy()
	runningAlloc.TaskStates = map[string]*structs.TaskState{
		task.Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
		task2.Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
	}
	err := b.Send(runningAlloc)
	require.NoError(t, err)

	// eventually, it is marked as healthy
	select {
	case <-time.After(4 * checkInterval):
		require.Fail(t, "timed out while waiting for health")
	case h := <-tracker.HealthyCh():
		require.True(t, h)
	}

}
