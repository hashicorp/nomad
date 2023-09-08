// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

// Statically assert the artifact hook implements the expected interface
var _ interfaces.TaskPrestartHook = (*artifactHook)(nil)

type mockEmitter struct {
	events []*structs.TaskEvent
}

func (m *mockEmitter) EmitEvent(ev *structs.TaskEvent) {
	m.events = append(m.events, ev)
}

// TestTaskRunner_ArtifactHook_Recoverable asserts that failures to download
// artifacts are a recoverable error.
func TestTaskRunner_ArtifactHook_Recoverable(t *testing.T) {
	ci.Parallel(t)

	me := &mockEmitter{}
	sbox := getter.TestSandbox(t)
	artifactHook := newArtifactHook(me, sbox, testlog.HCLogger(t))

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewEmptyTaskEnv(),
		TaskDir: &allocdir.TaskDir{Dir: os.TempDir()},
		Task: &structs.Task{
			Artifacts: []*structs.TaskArtifact{
				{
					GetterSource: "http://127.0.0.1:0",
					GetterMode:   structs.GetterModeAny,
				},
			},
		},
	}

	resp := interfaces.TaskPrestartResponse{}

	err := artifactHook.Prestart(context.Background(), req, &resp)

	require.False(t, resp.Done)
	require.NotNil(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)
}

// TestTaskRunnerArtifactHook_PartialDone asserts that the artifact hook skips
// already downloaded artifacts when subsequent artifacts fail and cause a
// restart.
func TestTaskRunner_ArtifactHook_PartialDone(t *testing.T) {
	testutil.RequireRoot(t)
	ci.Parallel(t)

	me := &mockEmitter{}
	sbox := getter.TestSandbox(t)
	artifactHook := newArtifactHook(me, sbox, testlog.HCLogger(t))

	// Create a source directory with 1 of the 2 artifacts
	srcdir := t.TempDir()

	// Only create one of the 2 artifacts to cause an error on first run.
	file1 := filepath.Join(srcdir, "foo.txt")
	require.NoError(t, os.WriteFile(file1, []byte{'1'}, 0644))

	// Test server to serve the artifacts
	ts := httptest.NewServer(http.FileServer(http.Dir(srcdir)))
	defer ts.Close()

	// Create the target directory.
	_, destdir := getter.SetupDir(t)

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewTaskEnv(nil, nil, nil, nil, destdir, ""),
		TaskDir: &allocdir.TaskDir{Dir: destdir},
		Task: &structs.Task{
			Artifacts: []*structs.TaskArtifact{
				{
					GetterSource: ts.URL + "/foo.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/bar.txt",
					GetterMode:   structs.GetterModeAny,
				},
			},
		},
	}

	resp := interfaces.TaskPrestartResponse{}

	// On first run file1 (foo) should download but file2 (bar) should
	// fail.
	err := artifactHook.Prestart(context.Background(), req, &resp)

	require.NotNil(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Len(t, resp.State, 1)
	require.False(t, resp.Done)
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)

	// Remove file1 from the server so it errors if its downloaded again.
	require.NoError(t, os.Remove(file1))

	// Write file2 so artifacts can download successfully
	file2 := filepath.Join(srcdir, "bar.txt")
	require.NoError(t, os.WriteFile(file2, []byte{'1'}, 0644))

	// Mock TaskRunner by copying state from resp to req and reset resp.
	req.PreviousState = maps.Clone(resp.State)

	resp = interfaces.TaskPrestartResponse{}

	// Retry the download and assert it succeeds
	err = artifactHook.Prestart(context.Background(), req, &resp)

	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.State, 2)

	// Assert both files downloaded properly
	files, err := filepath.Glob(filepath.Join(destdir, "*.txt"))
	require.NoError(t, err)
	sort.Strings(files)
	require.Contains(t, files[0], "bar.txt")
	require.Contains(t, files[1], "foo.txt")

	// Stop the test server entirely and assert that re-running works
	ts.Close()
	req.PreviousState = maps.Clone(resp.State)
	resp = interfaces.TaskPrestartResponse{}
	err = artifactHook.Prestart(context.Background(), req, &resp)
	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.State, 2)
}

// TestTaskRunner_ArtifactHook_ConcurrentDownloadSuccess asserts that the artifact hook
// download multiple files concurrently. this is a successful test without any errors.
func TestTaskRunner_ArtifactHook_ConcurrentDownloadSuccess(t *testing.T) {
	ci.SkipTestWithoutRootAccess(t)
	ci.Parallel(t)

	me := &mockEmitter{}
	sbox := getter.TestSandbox(t)
	artifactHook := newArtifactHook(me, sbox, testlog.HCLogger(t))

	// Create a source directory all 7 artifacts
	srcdir := t.TempDir()

	numOfFiles := 7
	for i := 0; i < numOfFiles; i++ {
		file := filepath.Join(srcdir, fmt.Sprintf("file%d.txt", i))
		require.NoError(t, os.WriteFile(file, []byte{byte(i)}, 0644))
	}

	// Test server to serve the artifacts
	ts := httptest.NewServer(http.FileServer(http.Dir(srcdir)))
	defer ts.Close()

	// Create the target directory.
	_, destdir := getter.SetupDir(t)

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewTaskEnv(nil, nil, nil, nil, destdir, ""),
		TaskDir: &allocdir.TaskDir{Dir: destdir},
		Task: &structs.Task{
			Artifacts: []*structs.TaskArtifact{
				{
					GetterSource: ts.URL + "/file0.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file1.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file2.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file3.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file4.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file5.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file6.txt",
					GetterMode:   structs.GetterModeAny,
				},
			},
		},
	}

	resp := interfaces.TaskPrestartResponse{}

	// start the hook
	err := artifactHook.Prestart(context.Background(), req, &resp)

	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.State, 7)
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)

	// Assert all files downloaded properly
	files, err := filepath.Glob(filepath.Join(destdir, "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 7)
	sort.Strings(files)
	require.Contains(t, files[0], "file0.txt")
	require.Contains(t, files[1], "file1.txt")
	require.Contains(t, files[2], "file2.txt")
	require.Contains(t, files[3], "file3.txt")
	require.Contains(t, files[4], "file4.txt")
	require.Contains(t, files[5], "file5.txt")
	require.Contains(t, files[6], "file6.txt")
}

// TestTaskRunner_ArtifactHook_ConcurrentDownloadFailure asserts that the artifact hook
// download multiple files concurrently. first iteration will result in failure and
// second iteration should succeed without downloading already downloaded files.
func TestTaskRunner_ArtifactHook_ConcurrentDownloadFailure(t *testing.T) {
	ci.Parallel(t)

	me := &mockEmitter{}
	sbox := getter.TestSandbox(t)
	artifactHook := newArtifactHook(me, sbox, testlog.HCLogger(t))

	// Create a source directory with 3 of the 4 artifacts
	srcdir := t.TempDir()

	file1 := filepath.Join(srcdir, "file1.txt")
	require.NoError(t, os.WriteFile(file1, []byte{'1'}, 0644))

	file2 := filepath.Join(srcdir, "file2.txt")
	require.NoError(t, os.WriteFile(file2, []byte{'2'}, 0644))

	file3 := filepath.Join(srcdir, "file3.txt")
	require.NoError(t, os.WriteFile(file3, []byte{'3'}, 0644))

	// Test server to serve the artifacts
	ts := httptest.NewServer(http.FileServer(http.Dir(srcdir)))
	defer ts.Close()

	// Create the target directory.
	_, destdir := getter.SetupDir(t)

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewTaskEnv(nil, nil, nil, nil, destdir, ""),
		TaskDir: &allocdir.TaskDir{Dir: destdir},
		Task: &structs.Task{
			Artifacts: []*structs.TaskArtifact{
				{
					GetterSource: ts.URL + "/file0.txt", // this request will fail
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file1.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file2.txt",
					GetterMode:   structs.GetterModeAny,
				},
				{
					GetterSource: ts.URL + "/file3.txt",
					GetterMode:   structs.GetterModeAny,
				},
			},
		},
	}

	resp := interfaces.TaskPrestartResponse{}

	// On first run all files will be downloaded except file0.txt
	err := artifactHook.Prestart(context.Background(), req, &resp)

	require.Error(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Len(t, resp.State, 3)
	require.False(t, resp.Done)
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)

	// delete the downloaded files so that it'll error if it's downloaded again
	require.NoError(t, os.Remove(file1))
	require.NoError(t, os.Remove(file2))
	require.NoError(t, os.Remove(file3))

	// create the missing file
	file0 := filepath.Join(srcdir, "file0.txt")
	require.NoError(t, os.WriteFile(file0, []byte{'0'}, 0644))

	// Mock TaskRunner by copying state from resp to req and reset resp.
	req.PreviousState = maps.Clone(resp.State)

	resp = interfaces.TaskPrestartResponse{}

	// Retry the download and assert it succeeds
	err = artifactHook.Prestart(context.Background(), req, &resp)
	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.State, 4)

	// Assert all files downloaded properly
	files, err := filepath.Glob(filepath.Join(destdir, "*.txt"))
	require.NoError(t, err)
	sort.Strings(files)
	require.Contains(t, files[0], "file0.txt")
	require.Contains(t, files[1], "file1.txt")
	require.Contains(t, files[2], "file2.txt")
	require.Contains(t, files[3], "file3.txt")

	// verify the file contents too, since files will also be created for failed downloads
	data0, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Equal(t, data0, []byte{'0'})

	data1, err := os.ReadFile(files[1])
	require.NoError(t, err)
	require.Equal(t, data1, []byte{'1'})

	data2, err := os.ReadFile(files[2])
	require.NoError(t, err)
	require.Equal(t, data2, []byte{'2'})

	data3, err := os.ReadFile(files[3])
	require.NoError(t, err)
	require.Equal(t, data3, []byte{'3'})

	require.True(t, resp.Done)
	require.Len(t, resp.State, 4)
}
