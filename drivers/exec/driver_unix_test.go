// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package exec

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	ctestutils "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestExecDriver_StartWaitStop(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources(allocID, "test"),
	}

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"600"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskConfig))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	defer harness.DestroyTask(task.ID, true)
	require.NoError(t, err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)

	require.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		harness.StopTask(task.ID, 2*time.Second, "SIGKILL")
	}()

	select {
	case result := <-ch:
		require.Equal(t, int(unix.SIGKILL), result.Signal)
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout waiting for task to shutdown")
	}

	// Ensure that the task is marked as dead, but account
	// for WaitTask() closing channel before internal state is updated
	testutil.WaitForResult(func() (bool, error) {
		status, err := harness.InspectTask(task.ID)
		if err != nil {
			return false, fmt.Errorf("inspecting task failed: %v", err)
		}
		if status.State != drivers.TaskStateExited {
			return false, fmt.Errorf("task hasn't exited yet; status: %v", status.State)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestExec_ExecTaskStreaming(t *testing.T) {
	ci.SkipTestWithoutRootAccess(t)
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		ID:        allocID,
		Name:      taskName,
		Resources: testResources(allocID, taskName),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"9000"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	_, _, err := harness.StartTask(task)
	require.NoError(t, err)
	defer d.DestroyTask(task.ID, true)

	dtestutil.ExecTaskStreamingConformanceTests(t, harness, task.ID)
}

// Tests that a given DNSConfig properly configures dns
func TestExec_dnsConfig(t *testing.T) {
	ci.Parallel(t)
	ctestutils.RequireRoot(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	cases := []struct {
		name string
		cfg  *drivers.DNSConfig
	}{
		{
			name: "nil DNSConfig",
		},
		{
			name: "basic",
			cfg: &drivers.DNSConfig{
				Servers: []string{"1.1.1.1", "1.0.0.1"},
			},
		},
		{
			name: "full",
			cfg: &drivers.DNSConfig{
				Servers:  []string{"1.1.1.1", "1.0.0.1"},
				Searches: []string{"local.test", "node.consul"},
				Options:  []string{"ndots:2", "edns0"},
			},
		},
	}

	for _, c := range cases {
		allocID := uuid.Generate()
		taskName := "sleep"
		task := &drivers.TaskConfig{
			ID:        allocID,
			Name:      taskName,
			DNS:       c.cfg,
			Resources: testResources(allocID, taskName),
		}

		cleanup := harness.MkAllocDir(task, false)
		defer cleanup()

		tc := &TaskConfig{
			Command: "/bin/sleep",
			Args:    []string{"9000"},
		}
		require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

		_, _, err := harness.StartTask(task)
		require.NoError(t, err)
		defer d.DestroyTask(task.ID, true)

		dtestutil.TestTaskDNSConfig(t, harness, task.ID, c.cfg)
	}
}

func TestExecDriver_Capabilities(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		ID:        allocID,
		Name:      taskName,
		Resources: testResources(allocID, taskName),
	}

	for _, tc := range []struct {
		Name       string
		CapAdd     []string
		CapDrop    []string
		AllowList  string
		StartError string
	}{
		{
			Name:    "default-allowlist-add-allowed",
			CapAdd:  []string{"fowner", "mknod"},
			CapDrop: []string{"ALL"},
		},
		{
			Name:       "default-allowlist-add-forbidden",
			CapAdd:     []string{"net_admin"},
			StartError: "net_admin",
		},
		{
			Name:    "default-allowlist-drop-existing",
			CapDrop: []string{"FOWNER", "MKNOD", "NET_RAW"},
		},
		{
			Name:      "restrictive-allowlist-drop-all",
			CapDrop:   []string{"ALL"},
			AllowList: "FOWNER,MKNOD",
		},
		{
			Name:      "restrictive-allowlist-add-allowed",
			CapAdd:    []string{"fowner", "mknod"},
			CapDrop:   []string{"ALL"},
			AllowList: "fowner,mknod",
		},
		{
			Name:       "restrictive-allowlist-add-forbidden",
			CapAdd:     []string{"net_admin", "mknod"},
			CapDrop:    []string{"ALL"},
			AllowList:  "fowner,mknod",
			StartError: "net_admin",
		},
		{
			Name:       "restrictive-allowlist-add-multiple-forbidden",
			CapAdd:     []string{"net_admin", "mknod", "CAP_SYS_TIME"},
			CapDrop:    []string{"ALL"},
			AllowList:  "fowner,mknod",
			StartError: "net_admin, sys_time",
		},
		{
			Name:      "permissive-allowlist",
			CapAdd:    []string{"net_admin", "mknod"},
			AllowList: "ALL",
		},
		{
			Name:      "permissive-allowlist-add-all",
			CapAdd:    []string{"all"},
			AllowList: "ALL",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			d := NewExecDriver(ctx, testlog.HCLogger(t))
			harness := dtestutil.NewDriverHarness(t, d)
			defer harness.Kill()

			config := &Config{
				NoPivotRoot:    true,
				DefaultModePID: executor.IsolationModePrivate,
				DefaultModeIPC: executor.IsolationModePrivate,
			}

			if tc.AllowList != "" {
				config.AllowCaps = strings.Split(tc.AllowList, ",")
			} else {
				// inherit HCL defaults if not set
				config.AllowCaps = capabilities.NomadDefaults().Slice(true)
			}

			var data []byte
			require.NoError(t, basePlug.MsgPackEncode(&data, config))
			baseConfig := &basePlug.Config{PluginConfig: data}
			require.NoError(t, harness.SetConfig(baseConfig))

			cleanup := harness.MkAllocDir(task, false)
			defer cleanup()

			tCfg := &TaskConfig{
				Command: "/bin/sleep",
				Args:    []string{"9000"},
			}
			if len(tc.CapAdd) > 0 {
				tCfg.CapAdd = tc.CapAdd
			}
			if len(tc.CapDrop) > 0 {
				tCfg.CapDrop = tc.CapDrop
			}
			require.NoError(t, task.EncodeConcreteDriverConfig(&tCfg))

			// check the start error against expectations
			_, _, err := harness.StartTask(task)
			if err == nil && tc.StartError != "" {
				t.Fatalf("Expected error in start: %v", tc.StartError)
			} else if err != nil {
				if tc.StartError == "" {
					require.NoError(t, err)
				} else {
					require.Contains(t, err.Error(), tc.StartError)
				}
				return
			}

			_ = d.DestroyTask(task.ID, true)
		})
	}
}
