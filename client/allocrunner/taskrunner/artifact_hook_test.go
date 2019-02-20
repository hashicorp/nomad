package taskrunner

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
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
	t.Parallel()

	me := &mockEmitter{}
	artifactHook := newArtifactHook(me, testlog.HCLogger(t))

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
	t.Parallel()

	me := &mockEmitter{}
	artifactHook := newArtifactHook(me, testlog.HCLogger(t))

	// Create a source directory with 1 of the 2 artifacts
	srcdir, err := ioutil.TempDir("", "nomadtest-src")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(srcdir))
	}()

	// Only create one of the 2 artifacts to cause an error on first run.
	file1 := filepath.Join(srcdir, "foo.txt")
	require.NoError(t, ioutil.WriteFile(file1, []byte{'1'}, 0644))

	// Test server to serve the artifacts
	ts := httptest.NewServer(http.FileServer(http.Dir(srcdir)))
	defer ts.Close()

	// Create the target directory.
	destdir, err := ioutil.TempDir("", "nomadtest-dest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(destdir))
	}()

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewEmptyTaskEnv(),
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
	err = artifactHook.Prestart(context.Background(), req, &resp)

	require.NotNil(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Len(t, resp.HookData, 1)
	require.False(t, resp.Done)
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)

	// Remove file1 from the server so it errors if its downloaded again.
	require.NoError(t, os.Remove(file1))

	// Write file2 so artifacts can download successfully
	file2 := filepath.Join(srcdir, "bar.txt")
	require.NoError(t, ioutil.WriteFile(file2, []byte{'1'}, 0644))

	// Mock TaskRunner by copying HookData from resp to req and reset resp.
	req.HookData = resp.HookData

	resp = interfaces.TaskPrestartResponse{}

	// Retry the download and assert it succeeds
	err = artifactHook.Prestart(context.Background(), req, &resp)

	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.HookData, 2)

	// Assert both files downloaded properly
	files, err := filepath.Glob(filepath.Join(destdir, "*.txt"))
	require.NoError(t, err)
	sort.Strings(files)
	require.Contains(t, files[0], "bar.txt")
	require.Contains(t, files[1], "foo.txt")

	// Stop the test server entirely and assert that re-running works
	ts.Close()
	req.HookData = resp.HookData
	resp = interfaces.TaskPrestartResponse{}
	err = artifactHook.Prestart(context.Background(), req, &resp)
	require.NoError(t, err)
	require.True(t, resp.Done)
	require.Len(t, resp.HookData, 2)
}
