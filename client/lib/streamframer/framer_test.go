// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framer

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
)

// This test checks, that even if the frame size has not been hit, a flush will
// periodically occur.
func TestStreamFramer_Flush(t *testing.T) {
	// Create the stream framer
	frames := make(chan *StreamFrame, 10)
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(frames, hRate, bWindow, 100)
	sf.Run()

	f := "foo"
	fe := "bar"
	d := []byte{0xa}
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			frame := <-frames

			if frame.IsHeartbeat() {
				continue
			}

			if reflect.DeepEqual(frame.Data, d) && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
				resultCh <- struct{}{}
				return
			}

		}
	}()

	// Write only 1 byte so we do not hit the frame size
	if err := sf.Send(f, fe, d, o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		t.Fatalf("failed to flush")
	}

	// Shutdown
	sf.Destroy()

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	if _, ok := <-frames; ok {
		t.Fatal("out channel should be closed")
	}
}

// This test checks that frames will be batched till the frame size is hit (in
// the case that is before the flush).
func TestStreamFramer_Batch(t *testing.T) {
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 500*time.Millisecond

	// Create the stream framer
	frames := make(chan *StreamFrame, 10)
	sf := NewStreamFramer(frames, hRate, bWindow, 3)
	sf.Run()

	f := "foo"
	fe := "bar"
	d := []byte{0xa, 0xb, 0xc}
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			frame := <-frames
			if frame.IsHeartbeat() {
				continue
			}

			if reflect.DeepEqual(frame.Data, d) && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	// Write only 1 byte so we do not hit the frame size
	if err := sf.Send(f, fe, d[:1], o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	// Ensure we didn't get any data
	select {
	case <-resultCh:
		t.Fatalf("Got data before frame size reached")
	case <-time.After(bWindow / 2):
	}

	// Write the rest so we hit the frame size
	if err := sf.Send(f, fe, d[1:], o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	// Ensure we get data
	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		t.Fatalf("Did not receive data after batch size reached")
	}

	// Shutdown
	sf.Destroy()

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	if f, ok := <-frames; ok {
		t.Fatalf("out channel should be closed. recv: %s", pretty.Sprint(f))
	}
}

func TestStreamFramer_Heartbeat(t *testing.T) {
	// Create the stream framer
	frames := make(chan *StreamFrame, 10)
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(frames, hRate, bWindow, 100)
	sf.Run()

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			frame := <-frames
			if frame.IsHeartbeat() {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("failed to heartbeat")
	}

	// Shutdown
	sf.Destroy()

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	if _, ok := <-frames; ok {
		t.Fatal("out channel should be closed")
	}
}

// This test checks that frames are received in order
func TestStreamFramer_Order(t *testing.T) {
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 10*time.Millisecond
	// Create the stream framer
	frames := make(chan *StreamFrame, 10)
	sf := NewStreamFramer(frames, hRate, bWindow, 10)
	sf.Run()

	files := []string{"1", "2", "3", "4", "5"}
	input := bytes.NewBuffer(make([]byte, 0, 100000))
	for i := 0; i <= 1000; i++ {
		str := strconv.Itoa(i) + ","
		input.WriteString(str)
	}

	expected := bytes.NewBuffer(make([]byte, 0, 100000))
	for range files {
		expected.Write(input.Bytes())
	}
	receivedBuf := bytes.NewBuffer(make([]byte, 0, 100000))

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			frame := <-frames
			if frame.IsHeartbeat() {
				continue
			}

			receivedBuf.Write(frame.Data)

			if reflect.DeepEqual(expected, receivedBuf) {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	// Send the data
	b := input.Bytes()
	shards := 10
	each := len(b) / shards
	for _, f := range files {
		for i := 0; i < shards; i++ {
			l, r := each*i, each*(i+1)
			if i == shards-1 {
				r = len(b)
			}

			if err := sf.Send(f, "", b[l:r], 0); err != nil {
				t.Fatalf("Send() failed %v", err)
			}
		}
	}

	// Ensure we get data
	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		if reflect.DeepEqual(expected, receivedBuf) {
			got := receivedBuf.String()
			want := expected.String()
			t.Fatalf("Got %v; want %v", got, want)
		}
	}

	// Shutdown
	sf.Destroy()

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	if _, ok := <-frames; ok {
		t.Fatal("out channel should be closed")
	}
}
