package base

import (
	"bytes"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
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

	// mock the RecoverTask driver call
	impl := &MockDriver{
		RecoverTaskF: func(h *TaskHandle) error {
			var actual testDriverState
			require.NoError(h.GetDriverState(&actual))
			require.Equal(state, actual)
			return nil
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()

	handle := &TaskHandle{
		driverState: buf.Bytes(),
	}
	err := harness.RecoverTask(handle)
	require.NoError(err)
}

func TestBaseDriver_StartTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	cfg := &TaskConfig{
		ID: "foo",
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

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()
	resp, err := harness.StartTask(cfg)
	require.NoError(err)
	require.Equal(cfg.ID, resp.Config.ID)
	require.Equal(handle.State, resp.State)

	var actualState testDriverState
	require.NoError(resp.GetDriverState(&actualState))
	require.Equal(*state, actualState)

}

func TestBaseDriver_WaitTask(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	result := &ExitResult{ExitCode: 1, Signal: 9}

	signalTask := make(chan struct{})

	impl := &MockDriver{
		WaitTaskF: func(_ context.Context, id string) chan *ExitResult {
			ch := make(chan *ExitResult)
			go func() {
				<-signalTask
				ch <- result
			}()
			return ch
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()
	var wg sync.WaitGroup
	wg.Add(1)
	var finished bool
	go func() {
		defer wg.Done()
		ch := harness.WaitTask(context.TODO(), "foo")
		actualResult := <-ch
		finished = true
		require.Exactly(result, actualResult)
	}()
	require.False(finished)
	close(signalTask)
	wg.Wait()
	require.True(finished)

}
