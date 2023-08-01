package taskrunner

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

// Statically assert the logmon hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*logmonHook)(nil)
var _ interfaces.TaskStopHook = (*logmonHook)(nil)

// TestTaskRunner_LogmonHook_LoadReattach unit tests loading logmon reattach
// config from persisted hook state.
func TestTaskRunner_LogmonHook_LoadReattach(t *testing.T) {
	ci.Parallel(t)

	// No hook data should return nothing
	cfg, err := reattachConfigFromHookData(nil)
	require.Nil(t, cfg)
	require.NoError(t, err)

	// Hook data without the appropriate key should return nothing
	cfg, err = reattachConfigFromHookData(map[string]string{"foo": "bar"})
	require.Nil(t, cfg)
	require.NoError(t, err)

	// Create a realistic reattach config and roundtrip it
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	orig := &plugin.ReattachConfig{
		Protocol: plugin.ProtocolGRPC,
		Addr:     addr,
		Pid:      4,
	}
	origJSON, err := json.Marshal(pstructs.ReattachConfigFromGoPlugin(orig))
	require.NoError(t, err)

	cfg, err = reattachConfigFromHookData(map[string]string{
		logmonReattachKey: string(origJSON),
	})
	require.NoError(t, err)

	require.Equal(t, orig, cfg)
}

// TestTaskRunner_LogmonHook_StartStop asserts that a new logmon is created the
// first time Prestart is called, reattached to on subsequent restarts, and
// killed on Stop.
func TestTaskRunner_LogmonHook_StartStop(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.BatchAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	dir := t.TempDir()

	hookConf := newLogMonHookConfig(task.Name, dir)
	runner := &TaskRunner{logmonHookConfig: hookConf}
	hook := newLogMonHook(runner, testlog.HCLogger(t))

	req := interfaces.TaskPrestartRequest{
		Task: task,
	}
	resp := interfaces.TaskPrestartResponse{}

	// First prestart should set reattach key but never be Done as it needs
	// to rerun on agent restarts to reattach.
	require.NoError(t, hook.Prestart(context.Background(), &req, &resp))
	defer hook.Stop(context.Background(), nil, nil)

	require.False(t, resp.Done)
	origHookData := resp.State[logmonReattachKey]
	require.NotEmpty(t, origHookData)

	// Running prestart again should effectively noop as it reattaches to
	// the running logmon.
	req.PreviousState = map[string]string{
		logmonReattachKey: origHookData,
	}
	require.NoError(t, hook.Prestart(context.Background(), &req, &resp))
	require.False(t, resp.Done)
	origHookData = resp.State[logmonReattachKey]
	require.Equal(t, origHookData, req.PreviousState[logmonReattachKey])

	// Running stop should shutdown logmon
	stopReq := interfaces.TaskStopRequest{
		ExistingState: maps.Clone(resp.State),
	}
	require.NoError(t, hook.Stop(context.Background(), &stopReq, nil))
}
