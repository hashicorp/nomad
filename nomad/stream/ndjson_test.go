// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

type testObj struct {
	Name string `json:"name"`
}

func TestJsonStream(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second)
	out := s.OutCh()

	require.NoError(t, s.Send(testObj{Name: "test"}))

	initialHeartbeat := <-out
	require.Equal(t, []byte(`{}`), initialHeartbeat.Data)

	testMessage1 := <-out
	require.Equal(t, []byte(`{"name":"test"}`), testMessage1.Data)

	select {
	case msg := <-out:
		require.Failf(t, "Did not expect another message", "%#v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, s.Send(testObj{Name: "test2"}))

	testMessage2 := <-out
	require.Equal(t, []byte(`{"name":"test2"}`), testMessage2.Data)
}

func TestJson_Send_After_Stop(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second)

	// stop the stream
	cancel()

	time.Sleep(10 * time.Millisecond)
	require.Error(t, s.Send(testObj{}))
}

func TestJson_HeartBeat(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 10*time.Millisecond)

	out := s.OutCh()
	heartbeat := <-out

	require.Equal(t, JsonHeartbeat, heartbeat)
}
