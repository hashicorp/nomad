package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/nomad/helper"
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
	parent, err := ioutil.TempDir(tmpDir, "nomadtest")
	if err != nil {
		t.Fatalf("error creating client dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(parent)
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

	conf.VaultConfig.Enabled = helper.BoolToPtr(false)
	conf.DevMode = true

	// Loosen GC threshold
	conf.GCDiskUsageThreshold = 98.0
	conf.GCInodeUsageThreshold = 98.0
	return conf, cleanup
}
