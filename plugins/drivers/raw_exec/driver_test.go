package raw_exec

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers/base"
	"github.com/stretchr/testify/require"
)

func TestDriverStartTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := base.NewDriverHarness(t, d)
	task := &base.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: "go",
		Args:    []string{"version"},
	})
	cleanup := harness.MkAllocDir(task)
	defer cleanup()

	handle, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Zero(result.ExitCode)
}
