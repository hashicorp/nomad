// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func artifactConfig(timeout time.Duration) *config.ArtifactConfig {
	return &config.ArtifactConfig{
		HTTPReadTimeout: timeout,
		HTTPMaxBytes:    1e6,
		GCSTimeout:      timeout,
		GitTimeout:      timeout,
		HgTimeout:       timeout,
		S3Timeout:       timeout,
	}
}

// comprehensive scenarios tested in e2e/artifact

func TestSandbox_Get_http(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
	}

	err := sbox.Get(env, artifact)
	must.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(taskDir, "local", "downloads", "go.mod"))
	must.NoError(t, err)
	must.StrContains(t, string(b), "module github.com/hashicorp/go-set")
}
