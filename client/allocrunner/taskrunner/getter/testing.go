// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"os"
	"path/filepath"
	"testing"

	cconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

// TestSandbox creates a real artifact downloader configured via the default
// artifact config. It is good enough for tests so no mock implementation exists.
func TestSandbox(t *testing.T) *Sandbox {
	defaultConfig := sconfig.DefaultArtifactConfig()
	defaultConfig.DecompressionSizeLimit = pointer.Of("1MB")
	defaultConfig.DecompressionFileCountLimit = pointer.Of(10)
	ac, err := cconfig.ArtifactConfigFromAgent(defaultConfig)
	must.NoError(t, err)
	return New(ac, testlog.HCLogger(t))
}

// SetupDir creates a directory suitable for testing artifact - i.e. it is
// owned by the user under which nomad runs.
//
// returns alloc_dir, task_dir
func SetupDir(t *testing.T) (string, string) {
	allocDir := t.TempDir()
	taskDir := filepath.Join(allocDir, "local")
	topDir := filepath.Dir(allocDir)

	must.NoError(t, os.Chmod(topDir, 0o755))

	must.NoError(t, os.Chmod(allocDir, 0o755))

	must.NoError(t, os.Mkdir(taskDir, 0o755))
	must.NoError(t, os.Chmod(taskDir, 0o755))
	return allocDir, taskDir
}
