package base

import (
	"bytes"
	"strings"
	"sync"
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
			DriverState: buf.Bytes(),
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
	require.Equal(req.Task.Id, resp.Handle.Config.Id)
	require.Equal(string(handle.State), string(strings.ToLower(resp.Handle.State.String())))

	dec := codec.NewDecoderBytes(resp.Handle.DriverState, structs.MsgpackHandle)
	var actualState testDriverState
	require.NoError(dec.Decode(&actualState))
	require.Equal(*state, actualState)

}

func TestBaseDriver_WaitTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	req := &proto.WaitTaskRequest{
		TaskId: "foo",
	}

	result := &TaskResult{ExitCode: 1, Signal: 9}

	signalTask := make(chan struct{})

	impl := &MockDriver{
		WaitTaskF: func(id string) chan *TaskResult {
			ch := make(chan *TaskResult)
			go func() {
				<-signalTask
				ch <- result
			}()
			return ch
		},
	}

	driver := &baseDriver{impl: impl}
	var wg sync.WaitGroup
	wg.Add(1)
	var finished bool
	go func() {
		defer wg.Done()
		resp, err := driver.WaitTask(context.TODO(), req)
		finished = true
		require.NoError(err)
		require.Equal(int(resp.Result.ExitCode), result.ExitCode)
		require.Equal(int(resp.Result.Signal), result.Signal)
	}()
	require.False(finished)
	close(signalTask)
	wg.Wait()
	require.True(finished)

}
