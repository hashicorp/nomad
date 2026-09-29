// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shoenig/test/must"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*dispatchHook)(nil)

// TestTaskRunner_DispatchHook_NoPayload asserts that the hook is a noop and is
// marked as done if there is no dispatch payload.
func TestTaskRunner_DispatchHook_NoPayload(t *testing.T) {
	ci.Parallel(t)

	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// Default mock alloc/job is not a dispatch job
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(logger, "nomadtest_nopayload", "nomadtest_nopayload", alloc.ID)
	defer allocDir.Destroy()
	taskDir := allocDir.NewTaskDir(task)
	must.NoError(t, taskDir.Build(fsisolation.None, nil, task.User))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}

	// Assert no error and Done=true as this job has no payload
	must.NoError(t, h.Prestart(ctx, &req, &resp))
	must.True(t, resp.Done)

	// Assert payload directory is empty
	files, err := os.ReadDir(req.TaskDir.LocalDir)
	must.NoError(t, err)
	must.SliceEmpty(t, files)
}

// TestTaskRunner_DispatchHook_Ok asserts that dispatch payloads are written to
// a file in the task dir.
func TestTaskRunner_DispatchHook_Ok(t *testing.T) {
	ci.Parallel(t)

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

	allocDir := allocdir.NewAllocDir(logger, "nomadtest_dispatchok", "nomadtest_dispatchok", alloc.ID)
	defer allocDir.Destroy()
	taskDir := allocDir.NewTaskDir(task)
	must.NoError(t, taskDir.Build(fsisolation.None, nil, task.User))

	h := newDispatchHook(alloc, logger)

	req := interfaces.TaskPrestartRequest{
		Task:    task,
		TaskDir: taskDir,
	}
	resp := interfaces.TaskPrestartResponse{}
	must.NoError(t, h.Prestart(ctx, &req, &resp))
	must.True(t, resp.Done)

	filename := filepath.Join(req.TaskDir.LocalDir, task.DispatchPayload.File)
	result, err := os.ReadFile(filename)
	must.NoError(t, err)
	must.Eq(t, expected, result)
}

// TestTaskRunner_DispatchHook_Error asserts that on an error dispatch payloads
// are not written and Done=false.
func TestTaskRunner_DispatchHook_Error(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name          string
		payload       []byte
		filePath      string
		symlink       string
		expectedError string
	}{
		{
			name:          "corrupt input",
			payload:       []byte("hello world"),
			filePath:      "out",
			expectedError: "snappy: corrupt input",
		},
		{
			name:          "symlink escape",
			payload:       snappy.Encode(nil, []byte("hello world")),
			filePath:      "tricksy-hobbits",
			symlink:       "../../tricksy-hobbitses",
			expectedError: "invalid value for dispatch_payload.file",
		},
		{
			name:          "relative escape",
			payload:       snappy.Encode(nil, []byte("hello world")),
			filePath:      "../secrets/tricksy-hobbits",
			expectedError: "invalid value for dispatch_payload.file",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			ctx := t.Context()
			logger := testlog.HCLogger(t)

			// Default mock alloc/job is not a dispatch job; update it
			alloc := mock.BatchAlloc()
			alloc.Job.ParameterizedJob = &structs.ParameterizedJobConfig{
				Payload: structs.DispatchPayloadRequired,
			}

			// Cause an error by not snappy encoding the payload
			alloc.Job.Payload = tc.payload

			// Set the filename and create the task dir
			task := alloc.Job.TaskGroups[0].Tasks[0]

			if tc.symlink != "" {
				task.DispatchPayload = &structs.DispatchPayloadConfig{File: tc.symlink}
			} else {
				task.DispatchPayload = &structs.DispatchPayloadConfig{File: tc.filePath}
			}

			tmp := t.TempDir()
			base := filepath.Join(tmp, "base_dir")
			taskDir := &allocdir.TaskDir{
				AllocDir: base,
				Dir:      filepath.Join(base, "task-name"),
				LocalDir: filepath.Join(base, "task-name", "local"),
			}
			// build needed directories
			must.NoError(t, os.Mkdir(base, 0755))
			must.NoError(t, os.Mkdir(taskDir.Dir, 0755))
			must.NoError(t, os.Mkdir(taskDir.LocalDir, 0755))

			// create a location outside the parent local dir and symlink
			// that directory and the local dir
			if tc.symlink != "" {
				escapeDir := filepath.Join(taskDir.AllocDir, "etc")
				must.NoError(t, os.Symlink(taskDir.LocalDir, escapeDir))
			}
			h := newDispatchHook(alloc, logger)

			req := interfaces.TaskPrestartRequest{
				Task:    task,
				TaskDir: taskDir,
			}
			resp := interfaces.TaskPrestartResponse{}

			// Assert an error was returned and Done=false
			must.EqError(t, h.Prestart(ctx, &req, &resp), tc.expectedError)

			must.False(t, resp.Done)

			// Assert payload directory is empty
			files, err := os.ReadDir(req.TaskDir.LocalDir)
			must.NoError(t, err)
			must.SliceEmpty(t, files)
		})
	}
}
