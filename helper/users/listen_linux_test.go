// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package users

import (
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

func TestListenTaskSocket_SELinuxLabel(t *testing.T) {
	expected, err := TaskSocketLabel()
	if err != nil || expected == "" {
		t.Skip("SELinux is disabled or has no container policy")
	}

	// The listening socket carries the container type, not the type of the
	// process that created it.
	ln := listenTaskSocket(t, true)
	must.Eq(t, expected, socketLabel(t, ln))
}

func TestListenTaskSocket_SELinuxLabelOff(t *testing.T) {
	if label, err := TaskSocketLabel(); err != nil || label == "" {
		t.Skip("SELinux is disabled or has no container policy")
	}

	// Without the option the socket keeps the label of the process that
	// created it.
	ln := listenTaskSocket(t, false)
	process, err := selinux.CurrentLabel()
	must.NoError(t, err)
	must.Eq(t, process, socketLabel(t, ln))
}

func TestListenWithSocketLabel_ResetsThread(t *testing.T) {
	socketLabel, err := TaskSocketLabel()
	if err != nil || socketLabel == "" {
		t.Skip("SELinux is disabled or has no container policy")
	}

	// Stay on one thread so the check below sees the thread the socket was
	// created on.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ln, reset, err := listenWithSocketLabel(filepath.Join(t.TempDir(), "test.sock"), socketLabel)
	must.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	must.True(t, reset)

	current, err := selinux.SocketLabel()
	must.NoError(t, err)
	must.Eq(t, "", current)
}

func listenTaskSocket(t *testing.T, selinuxLabel bool) net.Listener {
	t.Helper()
	ln, err := ListenTaskSocket(filepath.Join(t.TempDir(), "test.sock"), selinuxLabel)
	must.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func socketLabel(t *testing.T, ln net.Listener) string {
	t.Helper()
	f, err := ln.(*net.UnixListener).File()
	must.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	buf := make([]byte, 256)
	n, err := unix.Fgetxattr(int(f.Fd()), "security.selinux", buf)
	must.NoError(t, err)
	return strings.TrimRight(string(buf[:n]), "\x00")
}
