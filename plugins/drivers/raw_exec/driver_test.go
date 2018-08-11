package raw_exec

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers/base"
	"github.com/stretchr/testify/require"
)

func TestDriverStartTask(t *testing.T) {
	d := NewRawExecDriver()
	harness := base.NewDriverHarness(t, d)
	task := &base.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
		DriverConfig: map[string]interface{}{
			"command": "go",
			"args":    []string{"version"},
		},
	}
	cleanup := harness.MkAllocDir(task)
	defer cleanup()

	handle, err := harness.StartTask(task)
	require.NoError(t, err)

	ch := harness.WaitTask(context.Background(), handle.Config.ID)
	result := <-ch
	require.Zero(t, result.ExitCode)
}
