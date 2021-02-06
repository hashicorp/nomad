// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package exec

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
)

func TestExecDriver_StartWaitStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources,
	}

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"600"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&taskConfig))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	defer harness.DestroyTask(task.ID, true)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		harness.StopTask(task.ID, 2*time.Second, "SIGKILL")
	}()

	select {
	case result := <-ch:
		require.Equal(int(unix.SIGKILL), result.Signal)
	case <-time.After(10 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
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
		require.NoError(err)
	})
}

func TestExec_ExecTaskStreaming(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"9000"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))

	_, _, err := harness.StartTask(task)
	require.NoError(err)
	defer d.DestroyTask(task.ID, true)

	dtestutil.ExecTaskStreamingConformanceTests(t, harness, task.ID)

}

// Tests that a given DNSConfig properly configures dns
func TestExec_dnsConfig(t *testing.T) {
	t.Parallel()
	ctestutils.RequireRoot(t)
	ctestutils.ExecCompatible(t)
	require := require.New(t)

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
		task := &drivers.TaskConfig{
			ID:   uuid.Generate(),
			Name: "sleep",
			DNS:  c.cfg,
		}

		cleanup := harness.MkAllocDir(task, false)
		defer cleanup()

		tc := &TaskConfig{
			Command: "/bin/sleep",
			Args:    []string{"9000"},
		}
		require.NoError(task.EncodeConcreteDriverConfig(&tc))

		_, _, err := harness.StartTask(task)
		require.NoError(err)
		defer d.DestroyTask(task.ID, true)

		dtestutil.TestTaskDNSConfig(t, harness, task.ID, c.cfg)
	}

}
