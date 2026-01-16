// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

type testObj struct {
	Name string `json:"name"`
}

func TestJsonStream(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second, testlog.HCLogger(t))
	out := s.OutCh()

	must.NoError(t, s.Send(testObj{Name: "test"}))

	initialHeartbeat := <-out
	must.Eq(t, initialHeartbeat.Data, []byte(`{}`))

	testMessage1 := <-out
	must.Eq(t, testMessage1.Data, []byte(`{"name":"test"}`))

	select {
	case msg := <-out:
		must.Unreachable(t, must.Sprintf("Did not expect another message %#v", msg))
	case <-time.After(100 * time.Millisecond):
	}

	must.NoError(t, s.Send(testObj{Name: "test2"}))

	testMessage2 := <-out
	must.Eq(t, testMessage2.Data, []byte(`{"name":"test2"}`))
}

func TestJson_Send_After_Stop(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 1*time.Second, testlog.HCLogger(t))

	// stop the stream
	cancel()

	time.Sleep(10 * time.Millisecond)
	must.Error(t, s.Send(testObj{}))
}

func TestJson_HeartBeat(t *testing.T) {
	ci.Parallel(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewJsonStream(ctx, 10*time.Millisecond, testlog.HCLogger(t))

	out := s.OutCh()
	heartbeat := <-out

	must.Eq(t, heartbeat, JsonHeartbeat)
}

// Test that Send returns an error if the stream outbound channel is backpressured
// and messages can't be enqueued within the jsonSendTimeout window.
func TestJson_Send_Timeout(t *testing.T) {
	ci.Parallel(t)

	// shorten the timeout for test speed
	oldTimeout := jsonSendTimeout
	jsonSendTimeout = 100 * time.Millisecond
	defer func() { jsonSendTimeout = oldTimeout }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a long heartbeat so heartbeat doesn't interfere
	s := NewJsonStream(ctx, 1*time.Hour, testlog.HCLogger(t))

	// Do not drain the OutCh() so that the internal buffer will fill and Send
	// will eventually block and hit the timeout.
	done := make(chan error, 1)
	go func() {
		// Keep sending until we observe an error from Send
		for i := range 1000 {
			err := s.Send(testObj{Name: fmt.Sprintf("msg-%d", i)})
			if err != nil {
				done <- err
				return
			}
		}
		// If all sends succeeded (unlikely), signal nil
		done <- nil
	}()

	select {
	case err := <-done:
		must.Error(t, err, must.Sprint("expected Send to eventually return an error due to backpressure"))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Send to return due to backpressure")
	}
}

// Test that Send does not time out when the consumer is draining the OutCh()
// at a rate faster than the configured jsonSendTimeout.
func TestJson_Send_With_Draining(t *testing.T) {
	ci.Parallel(t)

	// shorten the timeout to make the test fast and deterministic
	oldTimeout := jsonSendTimeout
	jsonSendTimeout = 200 * time.Millisecond
	defer func() { jsonSendTimeout = oldTimeout }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create stream and start a consumer that drains at a modest rate
	s := NewJsonStream(ctx, 1*time.Hour, testlog.HCLogger(t))
	out := s.OutCh()

	// consumer: drain messages, sleeping a bit to simulate a
	// slow-but-not-stalled client
	consumeDone := make(chan struct{})
	go func() {
		defer close(consumeDone)
		for range 50 {
			// read and discard
			select {
			case <-out:
				// sleep less than jsonSendTimeout so producers should not hit
				// timeout
				time.Sleep(10 * time.Millisecond)
			case <-time.After(1 * time.Second):
				// unexpected stall
				return
			}
		}
	}()

	// produce many messages; with the consumer draining we should not get timeout
	prodErr := make(chan error, 1)
	go func() {
		for i := range 50 {
			if err := s.Send(testObj{Name: fmt.Sprintf("p-%d", i)}); err != nil {
				prodErr <- err
				return
			}
		}
		prodErr <- nil
	}()

	select {
	case err := <-prodErr:
		must.NoError(t, err, must.Sprint("did not expect Send to error when consumer is draining"))
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for producer to finish")
	}

	// ensure consumer finished
	select {
	case <-consumeDone:
	case <-time.After(1 * time.Second):
		t.Fatal("consumer did not finish as expected")
	}
}
