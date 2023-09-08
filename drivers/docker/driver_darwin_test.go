// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin

package docker

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestMain is a hacky test entrypoint to set temp directory to a path that can
// be mounted into Docker containers on macOS without needing dev performing
// special setup.
//
// macOS sets tempdir as `/var`, which Docker does not allowlist as a path that
// can be bind-mounted.
func TestMain(m *testing.M) {
	tmpdir := fmt.Sprintf("/tmp/nomad-docker-tests-%d", time.Now().Unix())

	os.Setenv("TMPDIR", os.Getenv("TMPDIR"))
	os.Setenv("TMPDIR", tmpdir)

	os.MkdirAll(tmpdir, 0700)
	defer os.RemoveAll(tmpdir)

	os.Exit(m.Run())
}
