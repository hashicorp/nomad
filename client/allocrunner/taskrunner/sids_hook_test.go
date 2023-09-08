// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

// todo(shoenig): Once Connect is supported on Windows, we'll need to make this
//  set of tests work there too.

package taskrunner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	consulapi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

var _ interfaces.TaskPrestartHook = (*sidsHook)(nil)

func sidecar(task string) (string, structs.TaskKind) {
	name := structs.ConnectProxyPrefix + "-" + task
	kind := structs.TaskKind(structs.ConnectProxyPrefix + ":" + task)
	return name, kind
}

func TestSIDSHook_recoverToken(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	secrets := t.TempDir()

	taskName, taskKind := sidecar("foo")
	h := newSIDSHook(sidsHookConfig{
		task: &structs.Task{
			Name: taskName,
			Kind: taskKind,
		},
		logger: testlog.HCLogger(t),
	})

	expected := uuid.Generate()
	err := h.writeToken(secrets, expected)
	r.NoError(err)

	token, err := h.recoverToken(secrets)
	r.NoError(err)
	r.Equal(expected, token)
}

func TestSIDSHook_recoverToken_empty(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	secrets := t.TempDir()

	taskName, taskKind := sidecar("foo")
	h := newSIDSHook(sidsHookConfig{
		task: &structs.Task{
			Name: taskName,
			Kind: taskKind,
		},
		logger: testlog.HCLogger(t),
	})

	token, err := h.recoverToken(secrets)
	r.NoError(err)
	r.Empty(token)
}

func TestSIDSHook_recoverToken_unReadable(t *testing.T) {
	ci.Parallel(t)
	// This test fails when running as root because the test case for checking
	// the error condition when the file is unreadable fails (root can read the
	// file even though the permissions are set to 0200).
	if unix.Geteuid() == 0 {
		t.Skip("test only works as non-root")
	}

	r := require.New(t)

	secrets := t.TempDir()

	err := os.Chmod(secrets, 0000)
	r.NoError(err)

	taskName, taskKind := sidecar("foo")
	h := newSIDSHook(sidsHookConfig{
		task: &structs.Task{
			Name: taskName,
			Kind: taskKind,
		},
		logger: testlog.HCLogger(t),
	})

	_, err = h.recoverToken(secrets)
	r.Error(err)
}

func TestSIDSHook_writeToken(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	secrets := t.TempDir()

	id := uuid.Generate()
	h := new(sidsHook)
	err := h.writeToken(secrets, id)
	r.NoError(err)

	content, err := os.ReadFile(filepath.Join(secrets, sidsTokenFile))
	r.NoError(err)
	r.Equal(id, string(content))
}

func TestSIDSHook_writeToken_unWritable(t *testing.T) {
	ci.Parallel(t)
	// This test fails when running as root because the test case for checking
	// the error condition when the file is unreadable fails (root can read the
	// file even though the permissions are set to 0200).
	if unix.Geteuid() == 0 {
		t.Skip("test only works as non-root")
	}

	r := require.New(t)

	secrets := t.TempDir()

	err := os.Chmod(secrets, 0000)
	r.NoError(err)

	id := uuid.Generate()
	h := new(sidsHook)
	err = h.writeToken(secrets, id)
	r.Error(err)
}

func Test_SIDSHook_writeToken_nonExistent(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	base := t.TempDir()
	secrets := filepath.Join(base, "does/not/exist")

	id := uuid.Generate()
	h := new(sidsHook)
	err := h.writeToken(secrets, id)
	r.Error(err)
}

func TestSIDSHook_deriveSIToken(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	taskName, taskKind := sidecar("task1")
	h := newSIDSHook(sidsHookConfig{
		alloc: &structs.Allocation{ID: "a1"},
		task: &structs.Task{
			Name: taskName,
			Kind: taskKind,
		},
		logger:     testlog.HCLogger(t),
		sidsClient: consulapi.NewMockServiceIdentitiesClient(),
	})

	ctx := context.Background()
	token, err := h.deriveSIToken(ctx)
	r.NoError(err)
	r.True(helper.IsUUID(token), "token: %q", token)
}

func TestSIDSHook_deriveSIToken_timeout(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	siClient := consulapi.NewMockServiceIdentitiesClient()
	siClient.DeriveTokenFn = func(allocation *structs.Allocation, strings []string) (m map[string]string, err error) {
		select {
		// block forever, hopefully triggering a timeout in the caller
		}
	}

	taskName, taskKind := sidecar("task1")
	h := newSIDSHook(sidsHookConfig{
		alloc: &structs.Allocation{ID: "a1"},
		task: &structs.Task{
			Name: taskName,
			Kind: taskKind,
		},
		logger:     testlog.HCLogger(t),
		sidsClient: siClient,
	})

	// set the timeout to a really small value for testing
	h.derivationTimeout = time.Duration(1 * time.Millisecond)

	ctx := context.Background()
	_, err := h.deriveSIToken(ctx)
	r.EqualError(err, "context deadline exceeded")
}

func TestSIDSHook_computeBackoff(t *testing.T) {
	ci.Parallel(t)

	try := func(i int, exp time.Duration) {
		result := computeBackoff(i)
		require.Equal(t, exp, result)
	}

	try(0, time.Duration(0))
	try(1, 100*time.Millisecond)
	try(2, 10*time.Second)
	try(3, 15*time.Second)
	try(4, 20*time.Second)
	try(5, 25*time.Second)
}

func TestSIDSHook_backoff(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	ctx := context.Background()
	stop := !backoff(ctx, 0)
	r.False(stop)
}

func TestSIDSHook_backoffKilled(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	stop := !backoff(ctx, 1000)
	r.True(stop)
}

func TestTaskRunner_DeriveSIToken_UnWritableTokenFile(t *testing.T) {
	ci.Parallel(t)
	// Normally this test would live in test_runner_test.go, but since it requires
	// root and the check for root doesn't like Windows, we put this file in here
	// for now.

	// This test fails when running as root because the test case for checking
	// the error condition when the file is unreadable fails (root can read the
	// file even though the permissions are set to 0200).
	if unix.Geteuid() == 0 {
		t.Skip("test only works as non-root")
	}

	r := require.New(t)

	alloc := mock.BatchConnectAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]interface{}{
		"run_for": "0s",
	}

	trConfig, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	// make the si_token file un-writable, triggering a failure after a
	// successful token derivation
	secrets := t.TempDir()
	trConfig.TaskDir.SecretsDir = secrets
	err := os.WriteFile(filepath.Join(secrets, sidsTokenFile), nil, 0400)
	r.NoError(err)

	// set a consul token for the nomad client, which is what triggers the
	// SIDS hook to be applied
	trConfig.ClientConfig.ConsulConfig.Token = uuid.Generate()

	// derive token works just fine
	deriveFn := func(*structs.Allocation, []string) (map[string]string, error) {
		return map[string]string{task.Name: uuid.Generate()}, nil
	}
	siClient := trConfig.ConsulSI.(*consulapi.MockServiceIdentitiesClient)
	siClient.DeriveTokenFn = deriveFn

	// start the task runner
	tr, err := NewTaskRunner(trConfig)
	r.NoError(err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	useMockEnvoyBootstrapHook(tr) // mock the envoy bootstrap

	go tr.Run()

	// wait for task runner to finish running
	testWaitForTaskToDie(t, tr)

	// assert task exited un-successfully
	finalState := tr.TaskState()
	r.Equal(structs.TaskStateDead, finalState.State)
	r.True(finalState.Failed) // should have failed to write SI token
	r.Contains(finalState.Events[2].DisplayMessage, "failed to write SI token")

	// assert the token is *not* on disk, as secrets dir was un-writable
	tokenPath := filepath.Join(trConfig.TaskDir.SecretsDir, sidsTokenFile)
	token, err := os.ReadFile(tokenPath)
	r.NoError(err)
	r.Empty(token)
}
