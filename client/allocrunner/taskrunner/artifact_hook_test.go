package taskrunner

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Statically assert the artifact hook implements the expected interface
var _ interfaces.TaskPrestartHook = (*artifactHook)(nil)

type mockEmitter struct {
	events []*structs.TaskEvent
}

func (m *mockEmitter) EmitEvent(ev *structs.TaskEvent) {
	m.events = append(m.events, ev)
}

// TestTaskRunner_ArtifactHook_Recoverable asserts that failures to download
// artifacts are a recoverable error.
func TestTaskRunner_ArtifactHook_Recoverable(t *testing.T) {
	t.Parallel()

	me := &mockEmitter{}
	artifactHook := newArtifactHook(me, testlog.HCLogger(t))

	req := &interfaces.TaskPrestartRequest{
		TaskEnv: taskenv.NewEmptyTaskEnv(),
		TaskDir: &allocdir.TaskDir{Dir: os.TempDir()},
		Task: &structs.Task{
			Artifacts: []*structs.TaskArtifact{
				{
					GetterSource: "http://127.0.0.1:0",
					GetterMode:   structs.GetterModeAny,
				},
			},
		},
	}

	resp := interfaces.TaskPrestartResponse{}

	err := artifactHook.Prestart(context.Background(), req, &resp)

	require.False(t, resp.Done)
	require.NotNil(t, err)
	require.True(t, structs.IsRecoverable(err))
	require.Len(t, me.events, 1)
	require.Equal(t, structs.TaskDownloadingArtifacts, me.events[0].Type)
}
