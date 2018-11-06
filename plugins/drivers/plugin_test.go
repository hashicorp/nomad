package drivers

import (
	"bytes"
	"sync"
	"testing"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
	"golang.org/x/net/context"
)

type testDriverState struct {
	Pid int
	Log string
}

func TestBaseDriver_Fingerprint(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	fingerprints := []*Fingerprint{
		{
			Attributes:        map[string]string{"foo": "bar"},
			Health:            HealthStateUnhealthy,
			HealthDescription: "starting up",
		},
		{
			Attributes:        map[string]string{"foo": "bar"},
			Health:            HealthStateHealthy,
			HealthDescription: "running",
		},
	}

	var complete bool
	impl := &MockDriver{
		FingerprintF: func(ctx context.Context) (<-chan *Fingerprint, error) {
			ch := make(chan *Fingerprint)
			go func() {
				defer close(ch)
				ch <- fingerprints[0]
				time.Sleep(500 * time.Millisecond)
				ch <- fingerprints[1]
				complete = true
			}()
			return ch, nil
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()

	ch, err := harness.Fingerprint(context.Background())
	require.NoError(err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case f := <-ch:
			require.Exactly(f, fingerprints[0])
		case <-time.After(1 * time.Second):
			require.Fail("did not receive fingerprint[0]")
		}
		select {
		case f := <-ch:
			require.Exactly(f, fingerprints[1])
		case <-time.After(1 * time.Second):
			require.Fail("did not receive fingerprint[1]")
		}
	}()
	require.False(complete)
	wg.Wait()
	require.True(complete)

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
		DriverState: buf.Bytes(),
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
		StartTaskF: func(c *TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error) {
			handle = NewTaskHandle("test")
			handle.Config = c
			handle.State = TaskStateRunning
			handle.SetDriverState(state)
			return handle, nil, nil
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()
	resp, _, err := harness.StartTask(cfg)
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
		WaitTaskF: func(_ context.Context, id string) (<-chan *ExitResult, error) {
			ch := make(chan *ExitResult)
			go func() {
				<-signalTask
				ch <- result
			}()
			return ch, nil
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()
	var wg sync.WaitGroup
	wg.Add(1)
	var finished bool
	go func() {
		defer wg.Done()
		ch, err := harness.WaitTask(context.TODO(), "foo")
		require.NoError(err)
		actualResult := <-ch
		finished = true
		require.Exactly(result, actualResult)
	}()
	require.False(finished)
	close(signalTask)
	wg.Wait()
	require.True(finished)
}

func TestBaseDriver_TaskEvents(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	events := []*TaskEvent{
		{
			TaskID:      "abc",
			Timestamp:   now,
			Annotations: map[string]string{"foo": "bar"},
			Message:     "starting",
		},
		{
			TaskID:      "xyz",
			Timestamp:   now.Add(2 * time.Second),
			Annotations: map[string]string{"foo": "bar"},
			Message:     "starting",
		},
		{
			TaskID:      "xyz",
			Timestamp:   now.Add(3 * time.Second),
			Annotations: map[string]string{"foo": "bar"},
			Message:     "running",
		},
		{
			TaskID:      "abc",
			Timestamp:   now.Add(4 * time.Second),
			Annotations: map[string]string{"foo": "bar"},
			Message:     "running",
		},
	}

	impl := &MockDriver{
		TaskEventsF: func(ctx context.Context) (<-chan *TaskEvent, error) {
			ch := make(chan *TaskEvent)
			go func() {
				defer close(ch)
				for _, event := range events {
					ch <- event
				}
			}()
			return ch, nil
		},
	}

	harness := NewDriverHarness(t, impl)
	defer harness.Kill()

	ch, err := harness.TaskEvents(context.Background())
	require.NoError(err)

	for _, event := range events {
		select {
		case actual := <-ch:
			require.Exactly(actual, event)
		case <-time.After(500 * time.Millisecond):
			require.Fail("failed to receive event")

		}
	}

}
