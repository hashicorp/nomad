// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestDockerDriver_authFromHelper(t *testing.T) {
	testutil.DockerCompatible(t)

	dir := t.TempDir()
	helperPayload := "{\"Username\":\"hashi\",\"Secret\":\"nomad\"}"
	helperContent := []byte(fmt.Sprintf("#!/bin/sh\ncat > %s/helper-$1.out;echo '%s'", dir, helperPayload))

	helperFile := filepath.Join(dir, "docker-credential-testnomad")
	err := os.WriteFile(helperFile, helperContent, 0777)
	must.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", path, dir))

	authHelper := authFromHelper("testnomad")
	creds, err := authHelper("registry.local:5000/repo/image")
	must.NoError(t, err)
	must.NotNil(t, creds)
	must.Eq(t, "hashi", creds.Username)
	must.Eq(t, "nomad", creds.Password)

	if _, err := os.Stat(filepath.Join(dir, "helper-get.out")); os.IsNotExist(err) {
		t.Fatalf("Expected helper-get.out to exist")
	}
	content, err := os.ReadFile(filepath.Join(dir, "helper-get.out"))
	must.NoError(t, err)
	must.Eq(t, "registry.local:5000", string(content))
}

func TestDockerDriver_PluginConfig_PidsLimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.PidsLimit = 5

	task, cfg, _ := dockerTask(t)
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	cfg.PidsLimit = 7
	_, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.Error(t, err)
	must.StrContains(t, err.Error(), `pids_limit cannot be greater than nomad plugin config pids_limit`)

	// Task PidsLimit should override plugin PidsLimit.
	cfg.PidsLimit = 3
	opts, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)
	must.Eq(t, pointer.Of(int64(3)), opts.Host.PidsLimit)
}

func TestDockerDriver_PidsLimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.PidsLimit = 1
	cfg.Command = "/bin/sh"
	cfg.Args = []string{"-c", "sleep 5 & sleep 5 & sleep 5"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	_, _, handle, cleanup := dockerSetup(t, task, nil)
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	select {
	case <-handle.waitCh:
		must.Eq(t, 2, handle.exitResult.ExitCode)
	case <-ctx.Done():
		t.Fatalf("task should have immediately completed")
	}

	// Check that data was written to the directory.
	outputFile := filepath.Join(task.TaskDir().LogDir, "redis-demo.stderr.0")
	exp := "can't fork"
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		act, err := os.ReadFile(outputFile)
		if err != nil {
			return err
		}
		if !strings.Contains(string(act), exp) {
			return fmt.Errorf("Expected %q in output %q", exp, string(act))
		}
		return nil
	}),
		wait.Timeout(5*time.Second),
		wait.Gap(50*time.Millisecond),
	))
}
