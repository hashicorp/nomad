// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	testing "github.com/mitchellh/go-testing-interface"
)

// TestClientConfig returns a default client configuration for test clients and
// a cleanup func to remove the state and alloc dirs when finished.
func TestClientConfig(t testing.T) (*Config, func()) {
	conf := DefaultConfig()
	conf.Node = mock.Node()
	conf.Logger = testlog.HCLogger(t)

	// On macOS, os.TempDir returns a symlinked path under /var which
	// is outside of the directories shared into the VM used for Docker.
	// Expand the symlink to get the real path in /private, which is ok.
	dirName := os.TempDir()
	tmpDir, err := filepath.EvalSymlinks(dirName)
	if err != nil {
		t.Fatalf("Could not resolve temporary directory links for %s: %v", tmpDir, err)
	}
	tmpDir = filepath.Clean(tmpDir)

	// Create a tempdir to hold state and alloc subdirs
	parent, err := os.MkdirTemp(tmpDir, "nomadtest")
	if err != nil {
		t.Fatalf("error creating client dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(parent)
	}

	// Fixup nomadtest dir permissions
	if err = os.Chmod(parent, 0777); err != nil {
		t.Fatalf("error updating permissions on nomadtest dir")
	}

	allocDir := filepath.Join(parent, "allocs")
	if err := os.Mkdir(allocDir, 0777); err != nil {
		cleanup()
		t.Fatalf("error creating alloc dir: %v", err)
	}
	conf.AllocDir = allocDir

	stateDir := filepath.Join(parent, "client")
	if err := os.Mkdir(stateDir, 0777); err != nil {
		cleanup()
		t.Fatalf("error creating alloc dir: %v", err)
	}
	conf.StateDir = stateDir

	// Use a minimal chroot environment
	conf.ChrootEnv = ci.TinyChroot

	// Helps make sure we are respecting configured parent
	conf.CgroupParent = "testing.slice"

	conf.VaultConfig.Enabled = pointer.Of(false)
	conf.VaultConfigs["default"].Enabled = pointer.Of(false)
	conf.DevMode = true

	// Loosen GC threshold
	conf.GCDiskUsageThreshold = 98.0
	conf.GCInodeUsageThreshold = 98.0

	// Same as default; necessary for task Event messages
	conf.MaxKillTimeout = 30 * time.Second

	// Provide a stub APIListenerRegistrar implementation
	conf.APIListenerRegistrar = NoopAPIListenerRegistrar{}

	return conf, cleanup
}

type NoopAPIListenerRegistrar struct{}

func (NoopAPIListenerRegistrar) Serve(_ context.Context, _ net.Listener) error {
	return nil
}
