// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package clientsockets

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/shoenig/test/must"
)

func TestConfined(t *testing.T) {
	ci.Parallel(t)

	must.True(t, Confined(fsisolation.Image))
	must.False(t, Confined(fsisolation.None))
	must.False(t, Confined(fsisolation.Chroot))
	must.False(t, Confined(fsisolation.Unveil))
	must.False(t, Confined(""))
}

// The tests below change the labeling state of the process, so they do not run
// in parallel.

func TestConfigure_Off(t *testing.T) {
	t.Cleanup(func() { _ = Configure(false) })
	must.NoError(t, Configure(false))

	label, active := Fingerprint()
	must.Eq(t, "", label)
	must.False(t, active)

	// Sockets for confined tasks are plain sockets that accept connections.
	path := filepath.Join(t.TempDir(), "test.sock")
	ln, err := ListenTaskSocket(path, true)
	must.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
		accepted <- err
	}()

	conn, err := net.Dial("unix", path)
	must.NoError(t, err)
	must.NoError(t, conn.Close())
	must.NoError(t, <-accepted)
}

func TestConfigure_NoSELinux(t *testing.T) {
	if selinux.GetEnabled() {
		t.Skip("SELinux is enabled")
	}
	t.Cleanup(func() { _ = Configure(false) })

	// The option is set but there is nothing to label with.
	must.NoError(t, Configure(true))
	label, active := Fingerprint()
	must.Eq(t, "", label)
	must.False(t, active)
}

func TestSocketLabeler_Stopped(t *testing.T) {
	ci.Parallel(t)

	// A request to a labeler that Configure stopped fails instead of waiting
	// for the goroutine, which no longer reads requests.
	l := &socketLabeler{requests: make(chan request), done: make(chan struct{})}
	close(l.done)

	path := filepath.Join(t.TempDir(), "test.sock")
	errCh := make(chan error, 1)
	go func() {
		_, err := l.listen(path)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		must.ErrorIs(t, err, errStopped)
	case <-time.After(10 * time.Second):
		t.Fatal("listen on a stopped labeler did not return")
	}
}
