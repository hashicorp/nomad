// +build !windows

package taskrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/require"
)

// TestTaskRunner_LogmonHook_StartCrashStop simulates logmon crashing while the
// Nomad client is restarting and asserts failing to reattach to logmon causes
// nomad to spawn a new logmon.
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

	origState := resp.State
	origHookData := resp.State[logmonReattachKey]
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
	req.PreviousState = map[string]string{
		logmonReattachKey: origHookData,
	}
	resp = interfaces.TaskPrestartResponse{}
	err = hook.Prestart(context.Background(), &req, &resp)
	require.NoError(t, err)
	require.NotEqual(t, origState, resp.State)

	// Running stop should shutdown logmon
	require.NoError(t, hook.Stop(context.Background(), nil, nil))
}

// TestTaskRunner_LogmonHook_ShutdownMidStart simulates logmon crashing while the
// Nomad client is calling Start() and asserts that we recover and spawn a new logmon.
func TestTaskRunner_LogmonHook_ShutdownMidStart(t *testing.T) {
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

	origState := resp.State
	origHookData := resp.State[logmonReattachKey]
	require.NotEmpty(t, origHookData)

	// Pluck PID out of reattach synthesize a crash
	reattach := struct {
		Pid int
	}{}
	require.NoError(t, json.Unmarshal([]byte(origHookData), &reattach))
	pid := reattach.Pid
	require.NotZero(t, pid)

	proc, err := process.NewProcess(int32(pid))
	require.NoError(t, err)

	// Assert logmon is running
	require.NoError(t, proc.SendSignal(syscall.Signal(0)))

	// SIGSTOP would freeze process without it being considered
	// exited; so this causes process to be non-exited at beginning of call
	// then we kill process while Start call is running
	require.NoError(t, proc.SendSignal(syscall.SIGSTOP))
	testutil.WaitForResult(func() (bool, error) {
		status, err := proc.Status()
		if err != nil {
			return false, err
		}

		if status != "T" && status != "T+" {
			return false, fmt.Errorf("process is not asleep yet: %v", status)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	go func() {
		time.Sleep(2 * time.Second)

		proc.SendSignal(syscall.SIGCONT)
		proc.Kill()
	}()

	req.PreviousState = map[string]string{
		logmonReattachKey: origHookData,
	}

	initLogmon, initClient := hook.logmon, hook.logmonPluginClient

	resp = interfaces.TaskPrestartResponse{}
	err = hook.Prestart(context.Background(), &req, &resp)
	require.NoError(t, err)
	require.NotEqual(t, origState, resp.State)

	// assert that we got a new client and logmon
	require.True(t, initLogmon != hook.logmon)
	require.True(t, initClient != hook.logmonPluginClient)
}
