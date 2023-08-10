// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestTaskRunner_DisableFileForVaultToken_UpgradePath(t *testing.T) {
	ci.Parallel(t)
	ci.SkipTestWithoutRootAccess(t)

	// Create test allocation with a Vault block.
	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Config = map[string]any{
		"run_for": "0s",
	}
	task.Vault = &structs.Vault{
		Policies: []string{"default"},
	}

	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()

	// Remove private dir and write the Vault token to the secrets dir to
	// simulate an old task.
	err := conf.TaskDir.Build(false, nil)
	must.NoError(t, err)

	err = syscall.Unmount(conf.TaskDir.PrivateDir, 0)
	must.NoError(t, err)
	err = os.Remove(conf.TaskDir.PrivateDir)
	must.NoError(t, err)

	token := "1234"
	tokenPath := filepath.Join(conf.TaskDir.SecretsDir, vaultTokenFile)
	err = os.WriteFile(tokenPath, []byte(token), 0666)
	must.NoError(t, err)

	// Setup a test Vault client.
	handler := func(*structs.Allocation, []string) (map[string]string, error) {
		return map[string]string{task.Name: token}, nil
	}
	vaultClient := conf.Vault.(*vaultclient.MockVaultClient)
	vaultClient.DeriveTokenFn = handler

	// Start task runner and wait for task to finish.
	tr, err := NewTaskRunner(conf)
	must.NoError(t, err)
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	go tr.Run()
	time.Sleep(500 * time.Millisecond)

	testWaitForTaskToDie(t, tr)

	// Verify task exited successfully.
	finalState := tr.TaskState()
	must.Eq(t, structs.TaskStateDead, finalState.State)
	must.False(t, finalState.Failed)

	// Verfiry token is in secrets dir.
	tokenPath = filepath.Join(conf.TaskDir.SecretsDir, vaultTokenFile)
	data, err := os.ReadFile(tokenPath)
	must.NoError(t, err)
	must.Eq(t, token, string(data))

	// Varify token is not in private dir since the allocation doesn't have
	// this path.
	tokenPath = filepath.Join(conf.TaskDir.PrivateDir, vaultTokenFile)
	_, err = os.Stat(tokenPath)
	must.ErrorIs(t, err, os.ErrNotExist)
}
