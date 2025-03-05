// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux
// +build linux

// todo(shoenig): Once Connect is supported on Windows, we'll need to make this
//  set of tests work there too.

package taskrunner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
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
