// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package users

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestListenTaskSocket(t *testing.T) {
	ci.Parallel(t)

	path := filepath.Join(t.TempDir(), "test.sock")
	ln, err := ListenTaskSocket(path, false)
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
