package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
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

func TestAllocDirFS_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// TODO This whole thing can go away since the ACLs should be tested in the
	// RPC test
	//for _, endpoint := range []string{"ls", "stat", "readat", "cat", "stream"} {
	for _, endpoint := range []string{"ls", "stat", "readat", "cat"} {
		t.Run(endpoint, func(t *testing.T) {

			httpACLTest(t, nil, func(s *TestAgent) {
				state := s.Agent.server.State()

				req, err := http.NewRequest("GET", fmt.Sprintf("/v1/client/fs/%s/", endpoint), nil)
				require.Nil(err)

				// Try request without a token and expect failure
				{
					respW := httptest.NewRecorder()
					_, err := s.Server.FsRequest(respW, req)
					require.NotNil(err)
					require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
				}

				// Try request with an invalid token and expect failure
				{
					respW := httptest.NewRecorder()
					policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadLogs})
					token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", policy)
					setToken(req, token)
					_, err := s.Server.FsRequest(respW, req)
					require.NotNil(err)
					require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
				}

				// Try request with a valid token
				// No alloc id set, so expect an error - just not a permissions error
				{
					respW := httptest.NewRecorder()
					policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadFS})
					token := mock.CreatePolicyAndToken(t, state, 1007, "valid", policy)
					setToken(req, token)
					_, err := s.Server.FsRequest(respW, req)
					require.NotNil(err)
					require.Equal(allocIDNotPresentErr, err)
				}

				// Try request with a management token
				// No alloc id set, so expect an error - just not a permissions error
				{
					respW := httptest.NewRecorder()
					setToken(req, s.RootToken)
					_, err := s.Server.FsRequest(respW, req)
					require.NotNil(err)
					require.Equal(allocIDNotPresentErr, err)
				}
			})
		})
	}
}

/*
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

		framer := sframer.NewStreamFramer(nopWriteCloser{ioutil.Discard}, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
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
				var frame sframer.StreamFrame
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

		framer := sframer.NewStreamFramer(w, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
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
				var frame sframer.StreamFrame
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

		framer := sframer.NewStreamFramer(w, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
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
				var frame sframer.StreamFrame
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

		framer := sframer.NewStreamFramer(wrappedW, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
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
				var frame sframer.StreamFrame
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
				var frame sframer.StreamFrame
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
*/
