package taskrunner

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
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
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)
	allocDir := allocdir.NewAllocDir(logger, "nomadtest_nopayload")
	defer allocDir.Destroy()

	// Default mock alloc/job is not a dispatch job
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil, cstructs.FSIsolationNone))

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
	files, err := ioutil.ReadDir(req.TaskDir.LocalDir)
	require.NoError(err)
	require.Empty(files)
}

// TestTaskRunner_DispatchHook_Ok asserts that dispatch payloads are written to
// a file in the task dir.
func TestTaskRunner_DispatchHook_Ok(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)
	allocDir := allocdir.NewAllocDir(logger, "nomadtest_dispatchok")
	defer allocDir.Destroy()

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
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil, cstructs.FSIsolationNone))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}
	require.NoError(h.Prestart(ctx, &req, &resp))
	require.True(resp.Done)

	filename := filepath.Join(req.TaskDir.LocalDir, task.DispatchPayload.File)
	result, err := ioutil.ReadFile(filename)
	require.NoError(err)
	require.Equal(expected, result)
}

// TestTaskRunner_DispatchHook_Error asserts that on an error dispatch payloads
// are not written and Done=false.
func TestTaskRunner_DispatchHook_Error(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()
	logger := testlog.HCLogger(t)
	allocDir := allocdir.NewAllocDir(logger, "nomadtest_dispatcherr")
	defer allocDir.Destroy()

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
	taskDir := allocDir.NewTaskDir(task.Name)
	require.NoError(taskDir.Build(false, nil, cstructs.FSIsolationNone))

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
	files, err := ioutil.ReadDir(req.TaskDir.LocalDir)
	require.NoError(err)
	require.Empty(files)
}
