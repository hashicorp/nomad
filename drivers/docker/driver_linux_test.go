// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestDockerDriver_authFromHelper(t *testing.T) {
	testutil.DockerCompatible(t)

	dir := t.TempDir()
	helperPayload := "{\"Username\":\"hashi\",\"Secret\":\"nomad\"}"
	helperContent := []byte(fmt.Sprintf("#!/bin/sh\ncat > %s/helper-$1.out;echo '%s'", dir, helperPayload))

	helperFile := filepath.Join(dir, "docker-credential-testnomad")
	err := os.WriteFile(helperFile, helperContent, 0777)
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", path, dir))

	authHelper := authFromHelper("testnomad")
	creds, err := authHelper("registry.local:5000/repo/image")
	require.NoError(t, err)
	require.NotNil(t, creds)
	require.Equal(t, "hashi", creds.Username)
	require.Equal(t, "nomad", creds.Password)

	if _, err := os.Stat(filepath.Join(dir, "helper-get.out")); os.IsNotExist(err) {
		t.Fatalf("Expected helper-get.out to exist")
	}
	content, err := os.ReadFile(filepath.Join(dir, "helper-get.out"))
	require.NoError(t, err)
	require.Equal(t, "registry.local:5000", string(content))
}

func TestDockerDriver_PluginConfig_PidsLimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.PidsLimit = 5

	task, cfg, _ := dockerTask(t)
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	cfg.PidsLimit = 7
	_, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), `pids_limit cannot be greater than nomad plugin config pids_limit`)

	// Task PidsLimit should override plugin PidsLimit.
	cfg.PidsLimit = 3
	opts, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)
	require.Equal(t, pointer.Of(int64(3)), opts.HostConfig.PidsLimit)
}

func TestDockerDriver_PidsLimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.PidsLimit = 1
	cfg.Command = "/bin/sh"
	cfg.Args = []string{"-c", "sleep 5 & sleep 5 & sleep 5"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	_, _, _, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	// Check that data was written to the directory.
	outputFile := filepath.Join(task.TaskDir().LogDir, "redis-demo.stderr.0")
	exp := "can't fork"
	tu.WaitForResult(func() (bool, error) {
		act, err := os.ReadFile(outputFile)
		if err != nil {
			return false, err
		}
		if !strings.Contains(string(act), exp) {
			return false, fmt.Errorf("Expected %q in output %q", exp, string(act))
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}
