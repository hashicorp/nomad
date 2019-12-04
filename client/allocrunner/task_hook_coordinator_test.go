package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

func TestTaskHookCoordinator_OnlyMainApp(t *testing.T) {
	alloc := mock.Alloc()
	tasks := alloc.Job.TaskGroups[0].Tasks
	logger := testlog.HCLogger(t)

	coord := newTaskHookCoordinator(logger, tasks)

	ch := coord.startConditionForTask(tasks[0])

	select {
	case _, ok := <-ch:
		require.False(t, ok)
	default:
		require.Fail(t, "channel wasn't closed")
	}
}
