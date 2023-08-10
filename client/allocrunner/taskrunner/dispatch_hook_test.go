// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*dispatchHook)(nil)

// TestTaskRunner_DispatchHook_NoPayload asserts that the hook is a noop and is
// marked as done if there is no dispatch payload.
func TestTaskRunner_DispatchHook_NoPayload(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// Default mock alloc/job is not a dispatch job
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(logger, "nomadtest_nopayload", alloc.ID)
	defer allocDir.Destroy()
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}

	// Assert no error and Done=true as this job has no payload
	require.NoError(h.Prestart(ctx, &req, &resp))
	require.True(resp.Done)

	// Assert payload directory is empty
	files, err := os.ReadDir(req.TaskDir.LocalDir)
	require.NoError(err)
	require.Empty(files)
}

// TestTaskRunner_DispatchHook_Ok asserts that dispatch payloads are written to
// a file in the task dir.
func TestTaskRunner_DispatchHook_Ok(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// Default mock alloc/job is not a dispatch job; update it
	alloc := mock.BatchAlloc()
	alloc.Job.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}
	expected := []byte("hello world")
	alloc.Job.Payload = snappy.Encode(nil, expected)

	// Set the filename and create the task dir
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.DispatchPayload = &structs.DispatchPayloadConfig{
		File: "out",
	}

	allocDir := allocdir.NewAllocDir(logger, "nomadtest_dispatchok", alloc.ID)
	defer allocDir.Destroy()
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}
	require.NoError(h.Prestart(ctx, &req, &resp))
	require.True(resp.Done)

	filename := filepath.Join(req.TaskDir.LocalDir, task.DispatchPayload.File)
	result, err := os.ReadFile(filename)
	require.NoError(err)
	require.Equal(expected, result)
}

// TestTaskRunner_DispatchHook_Error asserts that on an error dispatch payloads
// are not written and Done=false.
func TestTaskRunner_DispatchHook_Error(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// Default mock alloc/job is not a dispatch job; update it
	alloc := mock.BatchAlloc()
	alloc.Job.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}

	// Cause an error by not snappy encoding the payload
	alloc.Job.Payload = []byte("hello world")

	// Set the filename and create the task dir
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.DispatchPayload = &structs.DispatchPayloadConfig{
		File: "out",
	}

	allocDir := allocdir.NewAllocDir(logger, "nomadtest_dispatcherr", alloc.ID)
	defer allocDir.Destroy()
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}

	// Assert an error was returned and Done=false
	require.Error(h.Prestart(ctx, &req, &resp))
	require.False(resp.Done)

	// Assert payload directory is empty
	files, err := os.ReadDir(req.TaskDir.LocalDir)
	require.NoError(err)
	require.Empty(files)
}
