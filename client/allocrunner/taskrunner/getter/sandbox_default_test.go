// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package getter

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestSandbox_Get_chown(t *testing.T) {
	testutil.RequireRoot(t) // NOTE: required for chown call
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := SetupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
		Chown:        true,
	}

	err := sbox.Get(env, artifact, "nobody")
	must.NoError(t, err)

	info, err := os.Stat(filepath.Join(taskDir, "local", "downloads"))
	must.NoError(t, err)

	uid := info.Sys().(*syscall.Stat_t).Uid
	must.Eq(t, 65534, uid) // nobody's conventional uid
}

func TestSandbox_Get_inspection_NonWindows(t *testing.T) {
	// These tests disable filesystem isolation as the
	// artifact inspection is what is being tested.
	testutil.RequireRoot(t) // NOTE: required for chown call

	sandboxSetup := func() (string, *Sandbox, interfaces.EnvReplacer) {
		logger := testlog.HCLogger(t)
		ac := artifactConfig(10 * time.Second)
		sbox := New(ac, logger)
		_, taskDir := SetupDir(t)
		env := noopTaskEnv(taskDir)
		sbox.ac.DisableFilesystemIsolation = true

		return taskDir, sbox, env
	}

	t.Run("properly chowns destination", func(t *testing.T) {
		taskDir, sbox, env := sandboxSetup()
		src, _ := servTestFile(t, "test-file")

		artifact := &structs.TaskArtifact{
			GetterSource: src,
			RelativeDest: "local/downloads",
			Chown:        true,
		}

		err := sbox.Get(env, artifact, "nobody")
		must.NoError(t, err)

		info, err := os.Stat(filepath.Join(taskDir, "local", "downloads"))
		must.NoError(t, err)

		uid := info.Sys().(*syscall.Stat_t).Uid
		must.Eq(t, 65534, uid) // nobody's conventional uid

		info, err = os.Stat(filepath.Join(taskDir, "local", "downloads", "test-file"))
		must.NoError(t, err)

		uid = info.Sys().(*syscall.Stat_t).Uid
		must.Eq(t, 65534, uid) // nobody's conventional uid
	})

}
