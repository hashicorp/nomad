package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/ugorji/go/codec"
)

func TestAllocDirFS_List_MissingParams(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
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
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa}
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

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
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
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 3)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa, 0xb, 0xc}
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

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
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
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

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
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("failed to heartbeat")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames are received in order
func TestStreamFramer_Order(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 10*time.Millisecond
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 10)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	files := []string{"1", "2", "3", "4", "5"}
	input := bytes.NewBuffer(make([]byte, 0, 100000))
	for i := 0; i <= 1000; i++ {
		str := strconv.Itoa(i) + ","
		input.WriteString(str)
	}

	expected := bytes.NewBuffer(make([]byte, 0, 100000))
	for _, _ = range files {
		expected.Write(input.Bytes())
	}
	receivedBuf := bytes.NewBuffer(make([]byte, 0, 100000))

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

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames are received in order
func TestStreamFramer_Order_PlainText(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 10*time.Millisecond
	sf := NewStreamFramer(wrappedW, true, hRate, bWindow, 10)
	sf.Run()

	files := []string{"1", "2", "3", "4", "5"}
	input := bytes.NewBuffer(make([]byte, 0, 100000))
	for i := 0; i <= 1000; i++ {
		str := strconv.Itoa(i) + ","
		input.WriteString(str)
	}

	expected := bytes.NewBuffer(make([]byte, 0, 100000))
	for _, _ = range files {
		expected.Write(input.Bytes())
	}
	receivedBuf := bytes.NewBuffer(make([]byte, 0, 100000))

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
	OUTER:
		for {
			if _, err := receivedBuf.ReadFrom(r); err != nil {
				if strings.Contains(err.Error(), "closed pipe") {
					resultCh <- struct{}{}
					return
				}
				t.Fatalf("bad read: %v", err)
			}

			if expected.Len() != receivedBuf.Len() {
				continue
			}
			expectedBytes := expected.Bytes()
			actualBytes := receivedBuf.Bytes()
			for i, e := range expectedBytes {
				if a := actualBytes[i]; a != e {
					continue OUTER
				}
			}
			resultCh <- struct{}{}
			return

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
		if expected.Len() != receivedBuf.Len() {
			t.Fatalf("Got %v; want %v", expected.Len(), receivedBuf.Len())
		}
		expectedBytes := expected.Bytes()
		actualBytes := receivedBuf.Bytes()
		for i, e := range expectedBytes {
			if a := actualBytes[i]; a != e {
				t.Fatalf("Index %d; Got %q; want %q", i, a, e)
			}
		}
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

func TestHTTP_Stream_MissingParams(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stream/", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stream/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}

		req, err = http.NewRequest("GET", "/v1/client/fs/stream/foo?path=/path/to/file", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// tempAllocDir returns a new alloc dir that is rooted in a temp dir. The caller
// should destroy the temp dir.
func tempAllocDir(t testing.TB) *allocdir.AllocDir {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}

	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}

	return allocdir.NewAllocDir(log.New(os.Stderr, "", log.LstdFlags), dir)
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func TestHTTP_Stream_NoFile(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		framer := NewStreamFramer(nopWriteCloser{ioutil.Discard}, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
		framer.Run()
		defer framer.Destroy()

		if err := s.Server.stream(0, "foo", ad, framer, nil); err == nil {
			t.Fatalf("expected an error when streaming unknown file")
		}
	})
}

func TestHTTP_Stream_Modify(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		// Create a file in the temp dir
		streamFile := "stream_file"
		f, err := os.Create(filepath.Join(ad.AllocDir, streamFile))
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		defer f.Close()

		// Create a decoder
		r, w := io.Pipe()
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		data := []byte("helloworld")

		// Start the reader
		resultCh := make(chan struct{})
		go func() {
			var collected []byte
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				collected = append(collected, frame.Data...)
				if reflect.DeepEqual(data, collected) {
					resultCh <- struct{}{}
					return
				}
			}
		}()

		// Write a few bytes
		if _, err := f.Write(data[:3]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		framer := NewStreamFramer(w, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
		framer.Run()
		defer framer.Destroy()

		// Start streaming
		go func() {
			if err := s.Server.stream(0, streamFile, ad, framer, nil); err != nil {
				t.Fatalf("stream() failed: %v", err)
			}
		}()

		// Sleep a little before writing more. This lets us check if the watch
		// is working.
		time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
		if _, err := f.Write(data[3:]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		select {
		case <-resultCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("failed to send new data")
		}
	})
}

func TestHTTP_Stream_Truncate(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		// Create a file in the temp dir
		streamFile := "stream_file"
		streamFilePath := filepath.Join(ad.AllocDir, streamFile)
		f, err := os.Create(streamFilePath)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		defer f.Close()

		// Create a decoder
		r, w := io.Pipe()
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		data := []byte("helloworld")

		// Start the reader
		truncateCh := make(chan struct{})
		dataPostTruncCh := make(chan struct{})
		go func() {
			var collected []byte
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				if frame.FileEvent == truncateEvent {
					close(truncateCh)
				}

				collected = append(collected, frame.Data...)
				if reflect.DeepEqual(data, collected) {
					close(dataPostTruncCh)
					return
				}
			}
		}()

		// Write a few bytes
		if _, err := f.Write(data[:3]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		framer := NewStreamFramer(w, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
		framer.Run()
		defer framer.Destroy()

		// Start streaming
		go func() {
			if err := s.Server.stream(0, streamFile, ad, framer, nil); err != nil {
				t.Fatalf("stream() failed: %v", err)
			}
		}()

		// Sleep a little before truncating. This lets us check if the watch
		// is working.
		time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
		if err := f.Truncate(0); err != nil {
			t.Fatalf("truncate failed: %v", err)
		}
		if err := f.Sync(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("failed to close file: %v", err)
		}

		f2, err := os.OpenFile(streamFilePath, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("failed to reopen file: %v", err)
		}
		defer f2.Close()
		if _, err := f2.Write(data[3:5]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		select {
		case <-truncateCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive truncate")
		}

		// Sleep a little before writing more. This lets us check if the watch
		// is working.
		time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
		if _, err := f2.Write(data[5:]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		select {
		case <-dataPostTruncCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive post truncate data")
		}
	})
}

func TestHTTP_Stream_Delete(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		// Create a file in the temp dir
		streamFile := "stream_file"
		streamFilePath := filepath.Join(ad.AllocDir, streamFile)
		f, err := os.Create(streamFilePath)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		defer f.Close()

		// Create a decoder
		r, w := io.Pipe()
		wrappedW := &WriteCloseChecker{WriteCloser: w}
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		data := []byte("helloworld")

		// Start the reader
		deleteCh := make(chan struct{})
		go func() {
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				if frame.FileEvent == deleteEvent {
					close(deleteCh)
					return
				}
			}
		}()

		// Write a few bytes
		if _, err := f.Write(data[:3]); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		framer := NewStreamFramer(wrappedW, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
		framer.Run()

		// Start streaming
		go func() {
			if err := s.Server.stream(0, streamFile, ad, framer, nil); err != nil {
				t.Fatalf("stream() failed: %v", err)
			}
		}()

		// Sleep a little before deleting. This lets us check if the watch
		// is working.
		time.Sleep(1 * time.Duration(testutil.TestMultiplier()) * time.Second)
		if err := os.Remove(streamFilePath); err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		select {
		case <-deleteCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive delete")
		}

		framer.Destroy()
		testutil.WaitForResult(func() (bool, error) {
			return wrappedW.Closed, nil
		}, func(err error) {
			t.Fatalf("connection not closed")
		})

	})
}

func TestHTTP_Logs_NoFollow(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir and create the log dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		logDir := filepath.Join(ad.SharedDir, allocdir.LogDirName)
		if err := os.MkdirAll(logDir, 0777); err != nil {
			t.Fatalf("Failed to make log dir: %v", err)
		}

		// Create a series of log files in the temp dir
		task := "foo"
		logType := "stdout"
		expected := []byte("012")
		for i := 0; i < 3; i++ {
			logFile := fmt.Sprintf("%s.%s.%d", task, logType, i)
			logFilePath := filepath.Join(logDir, logFile)
			err := ioutil.WriteFile(logFilePath, expected[i:i+1], 777)
			if err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}

		// Create a decoder
		r, w := io.Pipe()
		wrappedW := &WriteCloseChecker{WriteCloser: w}
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		var received []byte

		// Start the reader
		resultCh := make(chan struct{})
		go func() {
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					if err == io.EOF {
						t.Logf("EOF")
						return
					}

					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				received = append(received, frame.Data...)
				if reflect.DeepEqual(received, expected) {
					close(resultCh)
					return
				}
			}
		}()

		// Start streaming logs
		go func() {
			if err := s.Server.logs(false, false, 0, OriginStart, task, logType, ad, wrappedW); err != nil {
				t.Fatalf("logs() failed: %v", err)
			}
		}()

		select {
		case <-resultCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive data: got %q", string(received))
		}

		testutil.WaitForResult(func() (bool, error) {
			return wrappedW.Closed, nil
		}, func(err error) {
			t.Fatalf("connection not closed")
		})

	})
}

func TestHTTP_Logs_Follow(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Get a temp alloc dir and create the log dir
		ad := tempAllocDir(t)
		defer os.RemoveAll(ad.AllocDir)

		logDir := filepath.Join(ad.SharedDir, allocdir.LogDirName)
		if err := os.MkdirAll(logDir, 0777); err != nil {
			t.Fatalf("Failed to make log dir: %v", err)
		}

		// Create a series of log files in the temp dir
		task := "foo"
		logType := "stdout"
		expected := []byte("012345")
		initialWrites := 3

		writeToFile := func(index int, data []byte) {
			logFile := fmt.Sprintf("%s.%s.%d", task, logType, index)
			logFilePath := filepath.Join(logDir, logFile)
			err := ioutil.WriteFile(logFilePath, data, 777)
			if err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
		for i := 0; i < initialWrites; i++ {
			writeToFile(i, expected[i:i+1])
		}

		// Create a decoder
		r, w := io.Pipe()
		wrappedW := &WriteCloseChecker{WriteCloser: w}
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		var received []byte

		// Start the reader
		firstResultCh := make(chan struct{})
		fullResultCh := make(chan struct{})
		go func() {
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					if err == io.EOF {
						t.Logf("EOF")
						return
					}

					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				received = append(received, frame.Data...)
				if reflect.DeepEqual(received, expected[:initialWrites]) {
					close(firstResultCh)
				} else if reflect.DeepEqual(received, expected) {
					close(fullResultCh)
					return
				}
			}
		}()

		// Start streaming logs
		go func() {
			if err := s.Server.logs(true, false, 0, OriginStart, task, logType, ad, wrappedW); err != nil {
				t.Fatalf("logs() failed: %v", err)
			}
		}()

		select {
		case <-firstResultCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive data: got %q", string(received))
		}

		// We got the first chunk of data, write out the rest to the next file
		// at an index much ahead to check that it is following and detecting
		// skips
		skipTo := initialWrites + 10
		writeToFile(skipTo, expected[initialWrites:])

		select {
		case <-fullResultCh:
		case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * streamBatchWindow):
			t.Fatalf("did not receive data: got %q", string(received))
		}

		// Close the reader
		r.Close()

		testutil.WaitForResult(func() (bool, error) {
			return wrappedW.Closed, nil
		}, func(err error) {
			t.Fatalf("connection not closed")
		})
	})
}

func BenchmarkHTTP_Logs_Follow(t *testing.B) {
	runtime.MemProfileRate = 1

	s := makeHTTPServer(t, nil)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)

	// Get a temp alloc dir and create the log dir
	ad := tempAllocDir(t)
	s.Agent.logger.Printf("ALEX: LOG DIR: %q", ad.SharedDir)
	//defer os.RemoveAll(ad.AllocDir)

	logDir := filepath.Join(ad.SharedDir, allocdir.LogDirName)
	if err := os.MkdirAll(logDir, 0777); err != nil {
		t.Fatalf("Failed to make log dir: %v", err)
	}

	// Create a series of log files in the temp dir
	task := "foo"
	logType := "stdout"
	expected := make([]byte, 1024*1024*100)
	initialWrites := 3

	writeToFile := func(index int, data []byte) {
		logFile := fmt.Sprintf("%s.%s.%d", task, logType, index)
		logFilePath := filepath.Join(logDir, logFile)
		err := ioutil.WriteFile(logFilePath, data, 777)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	part := (len(expected) / 3) - 50
	goodEnough := (8 * len(expected)) / 10
	for i := 0; i < initialWrites; i++ {
		writeToFile(i, expected[i*part:(i+1)*part])
	}

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		s.Agent.logger.Printf("BENCHMARK %d", i)

		// Create a decoder
		r, w := io.Pipe()
		wrappedW := &WriteCloseChecker{WriteCloser: w}
		defer r.Close()
		defer w.Close()
		dec := codec.NewDecoder(r, structs.JsonHandle)

		var received []byte

		// Start the reader
		fullResultCh := make(chan struct{})
		go func() {
			for {
				var frame StreamFrame
				if err := dec.Decode(&frame); err != nil {
					if err == io.EOF {
						t.Logf("EOF")
						return
					}

					t.Fatalf("failed to decode: %v", err)
				}

				if frame.IsHeartbeat() {
					continue
				}

				received = append(received, frame.Data...)
				if len(received) > goodEnough {
					close(fullResultCh)
					return
				}
			}
		}()

		// Start streaming logs
		go func() {
			if err := s.Server.logs(true, false, 0, OriginStart, task, logType, ad, wrappedW); err != nil {
				t.Fatalf("logs() failed: %v", err)
			}
		}()

		select {
		case <-fullResultCh:
		case <-time.After(time.Duration(60 * time.Second)):
			t.Fatalf("did not receive data: %d < %d", len(received), goodEnough)
		}

		s.Agent.logger.Printf("ALEX: CLOSING")

		// Close the reader
		r.Close()
		s.Agent.logger.Printf("ALEX: CLOSED")

		s.Agent.logger.Printf("ALEX: WAITING FOR WRITER TO CLOSE")
		testutil.WaitForResult(func() (bool, error) {
			return wrappedW.Closed, nil
		}, func(err error) {
			t.Fatalf("connection not closed")
		})
		s.Agent.logger.Printf("ALEX: WRITER CLOSED")
	}
}

func TestLogs_findClosest(t *testing.T) {
	task := "foo"
	entries := []*allocdir.AllocFileInfo{
		{
			Name: "foo.stdout.0",
			Size: 100,
		},
		{
			Name: "foo.stdout.1",
			Size: 100,
		},
		{
			Name: "foo.stdout.2",
			Size: 100,
		},
		{
			Name: "foo.stdout.3",
			Size: 100,
		},
		{
			Name: "foo.stderr.0",
			Size: 100,
		},
		{
			Name: "foo.stderr.1",
			Size: 100,
		},
		{
			Name: "foo.stderr.2",
			Size: 100,
		},
	}

	cases := []struct {
		Entries        []*allocdir.AllocFileInfo
		DesiredIdx     int64
		DesiredOffset  int64
		Task           string
		LogType        string
		ExpectedFile   string
		ExpectedIdx    int64
		ExpectedOffset int64
		Error          bool
	}{
		// Test error cases
		{
			Entries:    nil,
			DesiredIdx: 0,
			Task:       task,
			LogType:    "stdout",
			Error:      true,
		},
		{
			Entries:    entries[0:3],
			DesiredIdx: 0,
			Task:       task,
			LogType:    "stderr",
			Error:      true,
		},

		// Test beginning cases
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[0].Name,
			ExpectedIdx:  0,
		},
		{
			// Desired offset should be ignored at edges
			Entries:        entries,
			DesiredIdx:     0,
			DesiredOffset:  -100,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[0].Name,
			ExpectedIdx:    0,
			ExpectedOffset: 0,
		},
		{
			// Desired offset should be ignored at edges
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  -1000,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[0].Name,
			ExpectedIdx:    0,
			ExpectedOffset: 0,
		},
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[4].Name,
			ExpectedIdx:  0,
		},
		{
			Entries:      entries,
			DesiredIdx:   0,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[0].Name,
			ExpectedIdx:  0,
		},

		// Test middle cases
		{
			Entries:      entries,
			DesiredIdx:   1,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[1].Name,
			ExpectedIdx:  1,
		},
		{
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  10,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[1].Name,
			ExpectedIdx:    1,
			ExpectedOffset: 10,
		},
		{
			Entries:        entries,
			DesiredIdx:     1,
			DesiredOffset:  110,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[2].Name,
			ExpectedIdx:    2,
			ExpectedOffset: 10,
		},
		{
			Entries:      entries,
			DesiredIdx:   1,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[5].Name,
			ExpectedIdx:  1,
		},
		// Test end cases
		{
			Entries:      entries,
			DesiredIdx:   math.MaxInt64,
			Task:         task,
			LogType:      "stdout",
			ExpectedFile: entries[3].Name,
			ExpectedIdx:  3,
		},
		{
			Entries:        entries,
			DesiredIdx:     math.MaxInt64,
			DesiredOffset:  math.MaxInt64,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[3].Name,
			ExpectedIdx:    3,
			ExpectedOffset: 100,
		},
		{
			Entries:        entries,
			DesiredIdx:     math.MaxInt64,
			DesiredOffset:  -10,
			Task:           task,
			LogType:        "stdout",
			ExpectedFile:   entries[3].Name,
			ExpectedIdx:    3,
			ExpectedOffset: 90,
		},
		{
			Entries:      entries,
			DesiredIdx:   math.MaxInt64,
			Task:         task,
			LogType:      "stderr",
			ExpectedFile: entries[6].Name,
			ExpectedIdx:  2,
		},
	}

	for i, c := range cases {
		entry, idx, offset, err := findClosest(c.Entries, c.DesiredIdx, c.DesiredOffset, c.Task, c.LogType)
		if err != nil {
			if !c.Error {
				t.Fatalf("case %d: Unexpected error: %v", i, err)
			}
			continue
		}

		if entry.Name != c.ExpectedFile {
			t.Fatalf("case %d: Got file %q; want %q", i, entry.Name, c.ExpectedFile)
		}
		if idx != c.ExpectedIdx {
			t.Fatalf("case %d: Got index %d; want %d", i, idx, c.ExpectedIdx)
		}
		if offset != c.ExpectedOffset {
			t.Fatalf("case %d: Got offset %d; want %d", i, offset, c.ExpectedOffset)
		}
	}
}
