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

func TestNDJson(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan *NDJson)
	s := NewNDJsonStream(out, 1*time.Second)
	s.Run(ctx)

	require.NoError(t, s.Send(testObj{Name: "test"}))

	out1 := <-out

	var expected bytes.Buffer
	expected.Write([]byte(`{"name":"test"}`))
	expected.Write([]byte("\n"))

	require.Equal(t, expected.Bytes(), out1.Data)
	select {
	case _ = <-out:
		t.Fatalf("Did not expect another message")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestNDJson_Send_After_Stop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan *NDJson)
	s := NewNDJsonStream(out, 1*time.Second)
	s.Run(ctx)

	// stop the stream
	cancel()

	time.Sleep(10 * time.Millisecond)
	require.Error(t, s.Send(testObj{}))
}

func TestNDJson_HeartBeat(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan *NDJson)
	s := NewNDJsonStream(out, 10*time.Millisecond)
	s.Run(ctx)

	heartbeat := <-out

	require.Equal(t, NDJsonHeartbeat, heartbeat)
}
