package stream

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testObj struct {
	Name string `json:"name"`
}

func TestJsonStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second)
	out := s.OutCh()

	require.NoError(t, s.Send(testObj{Name: "test"}))

	out1 := <-out

	var expected bytes.Buffer
	expected.Write([]byte(`{"name":"test"}`))

	require.Equal(t, expected.Bytes(), out1.Data)
	select {
	case msg := <-out:
		require.Failf(t, "Did not expect another message", "%#v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, s.Send(testObj{Name: "test2"}))

	out2 := <-out
	expected.Reset()

	expected.Write([]byte(`{"name":"test2"}`))
	require.Equal(t, expected.Bytes(), out2.Data)

}

func TestJson_Send_After_Stop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second)

	// stop the stream
	cancel()

	time.Sleep(10 * time.Millisecond)
	require.Error(t, s.Send(testObj{}))
}

func TestJson_HeartBeat(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 10*time.Millisecond)

	out := s.OutCh()
	heartbeat := <-out

	require.Equal(t, JsonHeartbeat, heartbeat)
}
