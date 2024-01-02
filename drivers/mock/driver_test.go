// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
)

func TestMockDriver_StartWaitRecoverWaitStop(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := testlog.HCLogger(t)
	d := NewMockDriver(ctx, logger).(*Driver)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	var data []byte
	must.NoError(t, basePlug.MsgPackEncode(&data, &Config{}))
	bconfig := &basePlug.Config{PluginConfig: data}
	must.NoError(t, harness.SetConfig(bconfig))

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "sleep",
		Env:     map[string]string{},
	}
	tc := &TaskConfig{
		Command: Command{
			RunFor:         "10s",
			runForDuration: time.Second * 10,
		},
		PluginExitAfter:         "30s",
		pluginExitAfterDuration: time.Second * 30,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	testtask.SetTaskConfigEnv(task)
	cleanup := mkTestAllocDir(t, harness, logger, task)
	t.Cleanup(cleanup)

	handle, _, err := harness.StartTask(task)
	must.NoError(t, err)

	ch, err := harness.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	var waitDone bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		must.Error(t, result.Err)
		waitDone = true
	}()

	originalStatus, err := d.InspectTask(task.ID)
	must.NoError(t, err)

	d.tasks.Delete(task.ID)

	wg.Wait()
	must.True(t, waitDone)
	_, err = d.InspectTask(task.ID)
	must.Eq(t, drivers.ErrTaskNotFound, err)

	err = d.RecoverTask(handle)
	must.NoError(t, err)

	// need to make sure the task is left running and doesn't just immediately
	// exit after we recover it
	must.Wait(t, wait.ContinualSuccess(
		wait.BoolFunc(func() bool {
			status, err := d.InspectTask(task.ID)
			must.NoError(t, err)
			return status.State == "running"
		}),
		wait.Timeout(1*time.Second),
		wait.Gap(100*time.Millisecond),
	))

	status, err := d.InspectTask(task.ID)
	must.NoError(t, err)
	must.Eq(t, originalStatus, status)

	ch, err = harness.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	wg.Add(1)
	waitDone = false
	go func() {
		defer wg.Done()
		result := <-ch
		must.NoError(t, result.Err)
		must.Zero(t, result.ExitCode)
		waitDone = true
	}()

	time.Sleep(300 * time.Millisecond)
	must.NoError(t, d.StopTask(task.ID, 0, "SIGKILL"))
	wg.Wait()
	must.NoError(t, d.DestroyTask(task.ID, false))
	must.True(t, waitDone)
}

func mkTestAllocDir(t *testing.T, h *dtestutil.DriverHarness, logger hclog.Logger, tc *drivers.TaskConfig) func() {
	dir, err := os.MkdirTemp("", "nomad_driver_harness-")
	must.NoError(t, err)

	allocDir := allocdir.NewAllocDir(logger, dir, tc.AllocID)
	must.NoError(t, allocDir.Build())

	tc.AllocDir = allocDir.AllocDir

	taskDir := allocDir.NewTaskDir(tc.Name)
	must.NoError(t, taskDir.Build(false, ci.TinyChroot))

	task := &structs.Task{
		Name: tc.Name,
		Env:  tc.Env,
	}

	// no logging
	tc.StdoutPath = os.DevNull
	tc.StderrPath = os.DevNull

	// Create the mock allocation
	alloc := mock.Alloc()
	alloc.ID = tc.AllocID
	if tc.Resources != nil {
		alloc.AllocatedResources.Tasks[task.Name] = tc.Resources.NomadResources
	}

	taskBuilder := taskenv.NewBuilder(mock.Node(), alloc, task, "global")
	dtestutil.SetEnvvars(taskBuilder, drivers.FSIsolationNone, taskDir)

	taskEnv := taskBuilder.Build()
	if tc.Env == nil {
		tc.Env = taskEnv.Map()
	} else {
		for k, v := range taskEnv.Map() {
			if _, ok := tc.Env[k]; !ok {
				tc.Env[k] = v
			}
		}
	}

	return func() {
		allocDir.Destroy()
	}
}
