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
	"time"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

// The tests in this file change the labeling state of the process, so they do
// not run in parallel.

func TestListenTaskSocket_SELinuxLabel(t *testing.T) {
	expected := requireLabeling(t)

	// The listening socket carries the container type, not the type of the
	// process that created it.
	ln := listenTaskSocket(t, true)
	must.Eq(t, expected, socketLabel(t, ln))
}

func TestListenTaskSocket_SELinuxLabelOff(t *testing.T) {
	requireLabeling(t)

	// A socket for a task that is not confined keeps the label of the process
	// that created it.
	ln := listenTaskSocket(t, false)
	process, err := selinux.CurrentLabel()
	must.NoError(t, err)
	must.Eq(t, process, socketLabel(t, ln))
}

func TestListenTaskSocket_SELinuxLabelConcurrent(t *testing.T) {
	expected := requireLabeling(t)

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

func TestConfigure_Twice(t *testing.T) {
	expected := requireLabeling(t)
	first := labeler.Load()

	// The label is unchanged, so the second call keeps the running labeler.
	must.NoError(t, Configure(true))
	must.True(t, first == labeler.Load())

	ln := listenTaskSocket(t, true)
	must.Eq(t, expected, socketLabel(t, ln))
}

func TestConfigure_Toggle(t *testing.T) {
	expected := requireLabeling(t)
	process, err := selinux.CurrentLabel()
	must.NoError(t, err)
	dir := t.TempDir()

	// configureAndListen runs Configure and creates a socket for a confined
	// task, failing the test if either hangs.
	configureAndListen := func(on bool, name string) net.Listener {
		t.Helper()
		type listenResult struct {
			ln  net.Listener
			err error
		}
		done := make(chan listenResult, 1)
		go func() {
			if err := Configure(on); err != nil {
				done <- listenResult{err: err}
				return
			}
			ln, err := ListenTaskSocket(filepath.Join(dir, name), true)
			done <- listenResult{ln: ln, err: err}
		}()
		select {
		case r := <-done:
			must.NoError(t, r.err)
			t.Cleanup(func() { _ = r.ln.Close() })
			return r.ln
		case <-time.After(10 * time.Second):
			t.Fatalf("Configure(%v) and ListenTaskSocket did not return", on)
			return nil
		}
	}

	ln := configureAndListen(true, "on.sock")
	must.Eq(t, expected, socketLabel(t, ln))
	first := labeler.Load()

	ln = configureAndListen(false, "off.sock")
	must.Eq(t, process, socketLabel(t, ln))
	label, active := Fingerprint()
	must.Eq(t, "", label)
	must.False(t, active)

	// Turning labeling off stops the old labeler, and a request that still
	// holds it fails.
	select {
	case <-first.done:
	default:
		t.Fatal("Configure(false) did not stop the labeler")
	}
	_, err = first.listen(filepath.Join(dir, "stopped.sock"))
	must.ErrorIs(t, err, errStopped)

	ln = configureAndListen(true, "on-again.sock")
	must.Eq(t, expected, socketLabel(t, ln))
	second := labeler.Load()
	must.True(t, first != second)

	// Turning it off again stops the second labeler as well.
	_ = configureAndListen(false, "off-again.sock")
	select {
	case <-second.done:
	default:
		t.Fatal("Configure(false) did not stop the second labeler")
	}
}

// requireLabeling enables labeling and returns the label sockets of confined
// tasks are created with. It skips the test when the host cannot label them.
func requireLabeling(t *testing.T) string {
	t.Helper()
	t.Cleanup(func() { _ = Configure(false) })
	if err := Configure(true); err != nil {
		t.Skipf("SELinux cannot label task sockets: %v", err)
	}
	label, active := Fingerprint()
	if !active {
		t.Skip("SELinux is disabled or has no container policy")
	}
	return label
}

func listenTaskSocket(t *testing.T, confined bool) net.Listener {
	t.Helper()
	ln, err := ListenTaskSocket(filepath.Join(t.TempDir(), "test.sock"), confined)
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
