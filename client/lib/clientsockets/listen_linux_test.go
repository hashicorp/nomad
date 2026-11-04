// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package clientsockets

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

func TestListenTaskSocket_SELinuxLabelConcurrent(t *testing.T) {
	expected, err := TaskSocketLabel()
	if err != nil || expected == "" {
		t.Skip("SELinux is disabled or has no container policy")
	}
	must.NoError(t, StartTaskSocketLabeler())

	// Sockets requested from many goroutines at once are all created with the
	// label by the one locked thread.
	dir := t.TempDir()
	lns := make([]net.Listener, 8)
	errs := make([]error, len(lns))
	var wg sync.WaitGroup
	for i := range lns {
		wg.Go(func() {
			lns[i], errs[i] = ListenTaskSocket(filepath.Join(dir, fmt.Sprintf("%d.sock", i)), true)
		})
	}
	wg.Wait()

	for i, ln := range lns {
		must.NoError(t, errs[i])
		t.Cleanup(func() { _ = ln.Close() })
		must.Eq(t, expected, socketLabel(t, ln))
	}

	// The calling thread never has a socket creation label set.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
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
