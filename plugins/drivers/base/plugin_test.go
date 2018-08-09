package base

import (
	"bytes"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
	"golang.org/x/net/context"
)

type testDriverState struct {
	Pid int
	Log string
}

func TestBaseDriver_RecoverTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// build driver state and encode it into proto msg
	state := testDriverState{Pid: 1, Log: "foo"}
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.MsgpackHandle)
	enc.Encode(state)
	req := &proto.RecoverTaskRequest{
		Handle: &proto.TaskHandle{
			Driver:             "test",
			MsgpackDriverState: buf.Bytes(),
		},
	}

	// mock the RecoverTask driver call
	impl := &MockDriver{
		RecoverTaskF: func(h *TaskHandle) error {
			var actual testDriverState
			require.NoError(h.GetDriverState(&actual))
			require.Equal(state, actual)
			return nil
		},
	}

	driver := &baseDriver{impl: impl}
	_, err := driver.RecoverTask(context.TODO(), req)
	require.NoError(err)
}

func TestBaseDriver_StartTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	req := &proto.StartTaskRequest{
		Task: &proto.TaskConfig{
			Id: "foo",
		},
	}
	state := &testDriverState{Pid: 1, Log: "log"}
	var handle *TaskHandle
	impl := &MockDriver{
		StartTaskF: func(c *TaskConfig) (*TaskHandle, error) {
			handle = NewTaskHandle("test")
			handle.Config = c
			handle.State = TaskStateRunning
			handle.SetDriverState(state)
			return handle, nil
		},
	}

	driver := &baseDriver{impl: impl}
	resp, err := driver.StartTask(context.TODO(), req)
	require.NoError(err)
	require.Equal(handle.Driver, resp.Handle.Driver)
	require.Equal(req.Task, resp.Handle.Config)
	require.Equal(string(handle.State), resp.Handle.State)

	dec := codec.NewDecoderBytes(resp.Handle.MsgpackDriverState, structs.MsgpackHandle)
	var actualState testDriverState
	require.NoError(dec.Decode(&actualState))
	require.Equal(*state, actualState)

}
