// +build !windows

package taskrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"syscall"
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// Statically assert the logmon hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*logmonHook)(nil)
var _ interfaces.TaskStopHook = (*logmonHook)(nil)

// TestTaskRunner_LogmonHook_LoadReattach unit tests loading logmon reattach
// config from persisted hook state.
func TestTaskRunner_LogmonHook_LoadReattach(t *testing.T) {
	t.Parallel()

	// No hook data should return nothing
	cfg, err := reattachConfigFromHookData(nil)
	require.Nil(t, cfg)
	require.NoError(t, err)

	// Hook data without the appropriate key should return nothing
	cfg, err = reattachConfigFromHookData(map[string]string{"foo": "bar"})
	require.Nil(t, cfg)
	require.NoError(t, err)

	// Create a realistic reattach config and roundtrip it
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	orig := &plugin.ReattachConfig{
		Protocol: plugin.ProtocolGRPC,
		Addr:     addr,
		Pid:      4,
	}
	origJSON, err := json.Marshal(pstructs.ReattachConfigFromGoPlugin(orig))
	require.NoError(t, err)

	cfg, err = reattachConfigFromHookData(map[string]string{
		logmonReattachKey: string(origJSON),
	})
	require.NoError(t, err)

	require.Equal(t, orig, cfg)
}

// TestTaskRunner_LogmonHook_StartStop asserts that a new logmon is created the
// first time Prestart is called, reattached to on subsequent restarts, and
// killed on Stop.
func TestTaskRunner_LogmonHook_StartStop(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	hookConf := newLogMonHookConfig(task.Name, dir)
	hook := newLogMonHook(hookConf, testlog.HCLogger(t))

	req := interfaces.TaskPrestartRequest{
		Task: task,
	}
	resp := interfaces.TaskPrestartResponse{}

	// First prestart should set reattach key but never be Done as it needs
	// to rerun on agent restarts to reattach.
	require.NoError(t, hook.Prestart(context.Background(), &req, &resp))
	defer hook.Stop(context.Background(), nil, nil)

	require.False(t, resp.Done)
	origHookData := resp.HookData[logmonReattachKey]
	require.NotEmpty(t, origHookData)

	// Running prestart again should effectively noop as it reattaches to
	// the running logmon.
	req.HookData = map[string]string{
		logmonReattachKey: origHookData,
	}
	require.NoError(t, hook.Prestart(context.Background(), &req, &resp))
	require.False(t, resp.Done)
	origHookData = resp.HookData[logmonReattachKey]
	require.Equal(t, origHookData, req.HookData[logmonReattachKey])

	// Running stop should shutdown logmon
	require.NoError(t, hook.Stop(context.Background(), nil, nil))
}

// TestTaskRunner_LogmonHook_StartCrashStop simulates logmon crashing while the
// Nomad client is restarting and asserts failing to reattach to logmon causes
// a recoverable error (task restart).
func TestTaskRunner_LogmonHook_StartCrashStop(t *testing.T) {
	t.Parallel()

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	dir, err := ioutil.TempDir("", "nomadtest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	hookConf := newLogMonHookConfig(task.Name, dir)
	hook := newLogMonHook(hookConf, testlog.HCLogger(t))

	req := interfaces.TaskPrestartRequest{
		Task: task,
	}
	resp := interfaces.TaskPrestartResponse{}

	// First start
	require.NoError(t, hook.Prestart(context.Background(), &req, &resp))
	defer hook.Stop(context.Background(), nil, nil)

	origHookData := resp.HookData[logmonReattachKey]
	require.NotEmpty(t, origHookData)

	// Pluck PID out of reattach synthesize a crash
	reattach := struct {
		Pid int
	}{}
	require.NoError(t, json.Unmarshal([]byte(origHookData), &reattach))
	pid := reattach.Pid
	require.NotZero(t, pid)

	proc, _ := os.FindProcess(pid)

	// Assert logmon is running
	require.NoError(t, proc.Signal(syscall.Signal(0)))

	// Kill it
	require.NoError(t, proc.Signal(os.Kill))

	// Since signals are asynchronous wait for the process to die
	testutil.WaitForResult(func() (bool, error) {
		err := proc.Signal(syscall.Signal(0))
		return err != nil, fmt.Errorf("pid %d still running", pid)
	}, func(err error) {
		require.NoError(t, err)
	})

	// Running prestart again should return a recoverable error with no
	// reattach config to cause the task to be restarted with a new logmon.
	req.HookData = map[string]string{
		logmonReattachKey: origHookData,
	}
	resp = interfaces.TaskPrestartResponse{}
	err = hook.Prestart(context.Background(), &req, &resp)
	require.Error(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Empty(t, resp.HookData)

	// Running stop should shutdown logmon
	require.NoError(t, hook.Stop(context.Background(), nil, nil))
}
