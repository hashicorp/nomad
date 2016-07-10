package agent

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ugorji/go/codec"
)

func TestAllocDirFS_List_MissingParams(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/ls/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.DirectoryListRequest(respW, req)
		if err != allocIDNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}
	})
}

func TestAllocDirFS_Stat_MissingParams(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stat/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err != allocIDNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		if err != fileNameNotPresentErr {
			t.Fatalf("expected err: %v, actual: %v", allocIDNotPresentErr, err)
		}

	})
}

func TestAllocDirFS_ReadAt_MissingParams(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		req, err := http.NewRequest("GET", "/v1/client/fs/readat/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo?path=/path/to/file", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

type WriteCloseChecker struct {
	io.WriteCloser
	Closed bool
}

func (w *WriteCloseChecker) Close() error {
	w.Closed = true
	return w.WriteCloser.Close()
}

// This test checks, that even if the frame size has not been hit, a flush will
// periodically occur.
func TestStreamFramer_Flush(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(wrappedW, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, jsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa}
	expected := base64.StdEncoding.EncodeToString(d)
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				continue
			}

			if frame.Data == expected && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
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
	case <-time.After(2 * bWindow):
		t.Fatalf("failed to flush")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(2 * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames will be batched till the frame size is hit (in
// the case that is before the flush).
func TestStreamFramer_Batch(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 500*time.Millisecond
	sf := NewStreamFramer(wrappedW, hRate, bWindow, 3)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, jsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa, 0xb, 0xc}
	expected := base64.StdEncoding.EncodeToString(d)
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				continue
			}

			if frame.Data == expected && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
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
	case <-time.After(2 * bWindow):
		t.Fatalf("Did not receive data after batch size reached")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(2 * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

func TestStreamFramer_Heartbeat(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(wrappedW, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, jsonHandle)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-resultCh:
	case <-time.After(2 * hRate):
		t.Fatalf("failed to heartbeat")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(2 * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}
