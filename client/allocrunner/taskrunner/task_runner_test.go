package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	consulapi "github.com/hashicorp/nomad/client/consul"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/shared/catalog"
	"github.com/hashicorp/nomad/plugins/shared/singleton"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockTaskStateUpdater struct {
	ch chan struct{}
}

func NewMockTaskStateUpdater() *MockTaskStateUpdater {
	return &MockTaskStateUpdater{
		ch: make(chan struct{}, 1),
	}
}

func (m *MockTaskStateUpdater) TaskStateUpdated() {
	select {
	case m.ch <- struct{}{}:
	default:
	}
}

// testTaskRunnerConfig returns a taskrunner.Config for the given alloc+task
// plus a cleanup func.
func testTaskRunnerConfig(t *testing.T, alloc *structs.Allocation, taskName string) (*Config, func()) {
	logger := testlog.HCLogger(t)
	pluginLoader := catalog.TestPluginLoader(t)
	clientConf, cleanup := config.TestClientConfig(t)

	// Find the task
	var thisTask *structs.Task
	for _, tg := range alloc.Job.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Name == taskName {
				if thisTask != nil {
					cleanup()
					t.Fatalf("multiple tasks named %q; cannot use this helper", taskName)
				}
				thisTask = task
			}
		}
	}
	if thisTask == nil {
		cleanup()
		t.Fatalf("could not find task %q", taskName)
	}

	// Create the alloc dir + task dir
	allocPath := filepath.Join(clientConf.AllocDir, alloc.ID)
	allocDir := allocdir.NewAllocDir(logger, allocPath)
	if err := allocDir.Build(); err != nil {
		cleanup()
		t.Fatalf("error building alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(taskName)

	trCleanup := func() {
		if err := allocDir.Destroy(); err != nil {
			t.Logf("error destroying alloc dir: %v", err)
		}
		cleanup()
	}

	conf := &Config{
		Alloc:                 alloc,
		ClientConfig:          clientConf,
		Consul:                consulapi.NewMockConsulServiceClient(t, logger),
		Task:                  thisTask,
		TaskDir:               taskDir,
		Logger:                clientConf.Logger,
		Vault:                 vaultclient.NewMockVaultClient(),
		StateDB:               cstate.NoopDB{},
		StateUpdater:          NewMockTaskStateUpdater(),
		PluginSingletonLoader: singleton.NewSingletonLoader(logger, pluginLoader),
	}
	return conf, trCleanup
}

// TestTaskRunner_Restore asserts restoring a running task does not rerun the
// task.
func TestTaskRunner_Restore_Running(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Name = "testtask"
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": 2 * time.Second,
	}
	conf, cleanup := testTaskRunnerConfig(t, alloc, "testtask")
	conf.StateDB = cstate.NewMemDB() // "persist" state between task runners
	defer cleanup()

	// Run the first TaskRunner
	origTR, err := NewTaskRunner(conf)
	require.NoError(err)
	go origTR.Run()
	defer origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for it to be running
	testutil.WaitForResult(func() (bool, error) {
		ts := origTR.TaskState()
		return ts.State == structs.TaskStateRunning, fmt.Errorf("%v", ts.State)
	}, func(err error) {
		t.Fatalf("expected running; got: %v", err)
	})

	// Cause TR to exit without shutting down task
	origTR.ctxCancel()
	<-origTR.WaitCh()

	// Start a new TaskRunner and make sure it does not rerun the task
	newTR, err := NewTaskRunner(conf)
	require.NoError(err)

	// Do the Restore
	require.NoError(newTR.Restore())

	go newTR.Run()
	defer newTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Wait for new task runner to exit when the process does
	<-newTR.WaitCh()

	// Assert that the process was only started once, and only restored once
	started := 0
	restored := 0
	state := newTR.TaskState()
	require.Equal(structs.TaskStateDead, state.State)
	for _, ev := range state.Events {
		t.Logf("task event: %s %s", ev.Type, ev.Message)
		switch ev.Type {
		case structs.TaskStarted:
			started++
		case structs.TaskRestored:
			restored++
		}
	}
	assert.Equal(t, 1, started)
	assert.Equal(t, 1, restored)
}
