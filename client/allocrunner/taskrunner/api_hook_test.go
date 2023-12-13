// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

type testAPIListenerRegistrar struct {
	cb func(net.Listener) error
}

func (n testAPIListenerRegistrar) Serve(_ context.Context, ln net.Listener) error {
	if n.cb != nil {
		return n.cb(ln)
	}
	return nil
}

// TestAPIHook_SoftFail asserts that the Task API Hook soft fails and does not
// return errors.
func TestAPIHook_SoftFail(t *testing.T) {
	ci.Parallel(t)

	// Use a SecretsDir that will always exceed Unix socket path length
	// limits (sun_path)
	dst := filepath.Join(t.TempDir(), strings.Repeat("_NOMAD_TEST_", 100))

	ctx := context.Background()
	srv := testAPIListenerRegistrar{}
	logger := testlog.HCLogger(t)
	h := newAPIHook(ctx, srv, logger)

	req := &interfaces.TaskPrestartRequest{
		Task: &structs.Task{}, // needs to be non-nil for Task.User lookup
		TaskDir: &allocdir.TaskDir{
			SecretsDir: dst,
		},
	}
	resp := &interfaces.TaskPrestartResponse{}

	err := h.Prestart(ctx, req, resp)
	must.NoError(t, err)

	// listener should not have been set
	must.Nil(t, h.ln)

	// File should not have been created
	_, err = os.Stat(dst)
	must.Error(t, err)

	// Assert stop also soft-fails
	stopReq := &interfaces.TaskStopRequest{
		TaskDir: req.TaskDir,
	}
	stopResp := &interfaces.TaskStopResponse{}
	err = h.Stop(ctx, stopReq, stopResp)
	must.NoError(t, err)

	// File should not have been created
	_, err = os.Stat(dst)
	must.Error(t, err)
}

// TestAPIHook_Ok asserts that the Task API Hook creates and cleans up a
// socket.
func TestAPIHook_Ok(t *testing.T) {
	ci.Parallel(t)

	// If this test fails it may be because TempDir() + /api.sock is longer than
	// the unix socket path length limit (sun_path) in which case the test should
	// use a different temporary directory on that platform.
	dst := t.TempDir()

	// Write "ok" and close the connection and listener
	srv := testAPIListenerRegistrar{
		cb: func(ln net.Listener) error {
			conn, err := ln.Accept()
			if err != nil {
				return err
			}
			if _, err = conn.Write([]byte("ok")); err != nil {
				return err
			}
			conn.Close()
			return nil
		},
	}

	ctx := context.Background()
	logger := testlog.HCLogger(t)
	h := newAPIHook(ctx, srv, logger)

	req := &interfaces.TaskPrestartRequest{
		Task: &structs.Task{
			User: "nobody",
		},
		TaskDir: &allocdir.TaskDir{
			SecretsDir: dst,
		},
	}
	resp := &interfaces.TaskPrestartResponse{}

	err := h.Prestart(ctx, req, resp)
	must.NoError(t, err)

	// File should have been created
	sockDst := apiSocketPath(req.TaskDir)

	// Stat and chown fail on Windows, so skip these checks
	if runtime.GOOS != "windows" {
		stat, err := os.Stat(sockDst)
		must.NoError(t, err)
		must.True(t, stat.Mode()&fs.ModeSocket != 0,
			must.Sprintf("expected %q to be a unix socket but got %s", sockDst, stat.Mode()))

		nobody, _ := users.Lookup("nobody")
		if syscall.Getuid() == 0 && nobody != nil {
			t.Logf("root and nobody exists: testing file perms")

			// We're root and nobody exists! Check perms
			must.Eq(t, fs.FileMode(0o600), stat.Mode().Perm())

			sysStat, ok := stat.Sys().(*syscall.Stat_t)
			must.True(t, ok, must.Sprintf("expected stat.Sys() to be a *syscall.Stat_t on %s but found %T",
				runtime.GOOS, stat.Sys()))

			nobodyUID, err := strconv.Atoi(nobody.Uid)
			must.NoError(t, err)
			must.Eq(t, nobodyUID, int(sysStat.Uid))
		}
	}

	// Assert the listener is working
	conn, err := net.Dial("unix", sockDst)
	must.NoError(t, err)
	buf := make([]byte, 2)
	_, err = conn.Read(buf)
	must.NoError(t, err)
	must.Eq(t, []byte("ok"), buf)
	conn.Close()

	// Assert stop cleans up
	stopReq := &interfaces.TaskStopRequest{
		TaskDir: req.TaskDir,
	}
	stopResp := &interfaces.TaskStopResponse{}
	err = h.Stop(ctx, stopReq, stopResp)
	must.NoError(t, err)

	// File should be gone
	_, err = net.Dial("unix", sockDst)
	must.Error(t, err)
}
