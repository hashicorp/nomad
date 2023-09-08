// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const (
	defaultLoggerMockDriverStdout = "Hello from the other side"
	xssLoggerMockDriverStdout     = "<script>alert(document.domain);</script>"
)

var (
	defaultLoggerMockDriver = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": defaultLoggerMockDriverStdout,
	}
	xssLoggerMockDriver = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": xssLoggerMockDriverStdout,
	}
)

type clientAllocWaiter int

const (
	noWaitClientAlloc clientAllocWaiter = iota
	runningClientAlloc
	terminalClientAlloc
)

func addAllocToClient(agent *TestAgent, alloc *structs.Allocation, wait clientAllocWaiter) {
	require := require.New(agent.T)

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		node, err := agent.server.State().NodeByID(nil, agent.client.NodeID())
		if err != nil {
			return false, err
		}
		if node == nil {
			return false, fmt.Errorf("unknown node")
		}

		return node.Status == structs.NodeStatusReady, fmt.Errorf("bad node status")
	}, func(err error) {
		agent.T.Fatal(err)
	})

	// Upsert the allocation
	state := agent.server.State()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc}))

	if wait == noWaitClientAlloc {
		return
	}

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, alloc.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}

		expectation := alloc.ClientStatus == structs.AllocClientStatusComplete ||
			alloc.ClientStatus == structs.AllocClientStatusFailed
		if wait == runningClientAlloc {
			expectation = expectation || alloc.ClientStatus == structs.AllocClientStatusRunning
		}

		if !expectation {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		agent.T.Fatal(err)
	})
}

// mockFSAlloc returns a suitable mock alloc for testing the fs system. If
// config isn't provided, the defaultLoggerMockDriver config is used.
func mockFSAlloc(nodeID string, config map[string]interface{}) *structs.Allocation {
	a := mock.Alloc()
	a.NodeID = nodeID
	a.Job.Type = structs.JobTypeBatch
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"

	if config != nil {
		a.Job.TaskGroups[0].Tasks[0].Config = config
	} else {
		a.Job.TaskGroups[0].Tasks[0].Config = defaultLoggerMockDriver
	}

	return a
}

func TestHTTP_FS_List_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/ls/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.DirectoryListRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())
	})
}

func TestHTTP_FS_Stat_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/stat/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/stat/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())
	})
}

func TestHTTP_FS_ReadAt_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/readat/", nil)
		require.NoError(err)

		_, err = s.Server.FileReadAtRequest(httptest.NewRecorder(), req)
		require.Error(err)

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/readat/foo", nil)
		require.NoError(err)

		_, err = s.Server.FileReadAtRequest(httptest.NewRecorder(), req)
		require.Error(err)

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/readat/foo?path=/path/to/file", nil)
		require.NoError(err)

		_, err = s.Server.FileReadAtRequest(httptest.NewRecorder(), req)
		require.Error(err)
	})
}

func TestHTTP_FS_Cat_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/cat/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.FileCatRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/stat/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileCatRequest(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())
	})
}

func TestHTTP_FS_Stream_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/stream/", nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/stream/foo", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())

		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/stream/foo?path=/path/to/file", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.Error(err)
		require.Contains(err.Error(), "alloc lookup failed")
	})
}

// TestHTTP_FS_Logs_MissingParams asserts proper error codes and messages are
// returned for incorrect parameters (eg missing tasks).
func TestHTTP_FS_Logs_MissingParams(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// AllocID Not Present
		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/logs/", nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		s.Server.mux.ServeHTTP(respW, req)
		require.Equal(respW.Body.String(), allocIDNotPresentErr.Error())
		require.Equal(400, respW.Code)

		// Task Not Present
		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/logs/foo", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		s.Server.mux.ServeHTTP(respW, req)
		require.Equal(respW.Body.String(), taskNotPresentErr.Error())
		require.Equal(400, respW.Code)

		// Log Type Not Present
		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/logs/foo?task=foo", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		s.Server.mux.ServeHTTP(respW, req)
		require.Equal(respW.Body.String(), logTypeNotPresentErr.Error())
		require.Equal(400, respW.Code)

		// case where all parameters are set but alloc isn't found
		req, err = http.NewRequest(http.MethodGet, "/v1/client/fs/logs/foo?task=foo&type=stdout", nil)
		require.NoError(err)
		respW = httptest.NewRecorder()

		s.Server.mux.ServeHTTP(respW, req)
		require.Equal(500, respW.Code)
		require.Contains(respW.Body.String(), "alloc lookup failed")
	})
}

func TestHTTP_FS_List(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		req, err := http.NewRequest(http.MethodGet, "/v1/client/fs/ls/"+a.ID, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		raw, err := s.Server.DirectoryListRequest(respW, req)
		require.Nil(err)

		files, ok := raw.([]*cstructs.AllocFileInfo)
		require.True(ok)
		require.NotEmpty(files)
		require.True(files[0].IsDir)
	})
}

func TestHTTP_FS_Stat(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("/v1/client/fs/stat/%s?path=alloc/", a.ID)
		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		raw, err := s.Server.FileStatRequest(respW, req)
		require.Nil(err)

		info, ok := raw.(*cstructs.AllocFileInfo)
		require.True(ok)
		require.NotNil(info)
		require.True(info.IsDir)
	})
}

func TestHTTP_FS_ReadAt(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 1
		limit := 3
		expectation := defaultLoggerMockDriverStdout[offset : offset+limit]
		path := fmt.Sprintf("/v1/client/fs/readat/%s?path=alloc/logs/web.stdout.0&offset=%d&limit=%d",
			a.ID, offset, limit)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.FileReadAtRequest(respW, req)
		require.Nil(err)

		output, err := io.ReadAll(respW.Result().Body)
		require.Nil(err)
		require.EqualValues(expectation, output)
	})
}

// TestHTTP_FS_ReadAt_XSS asserts that the readat API is safe from XSS.
func TestHTTP_FS_ReadAt_XSS(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), xssLoggerMockDriver)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("%s/v1/client/fs/readat/%s?path=alloc/logs/web.stdout.0&offset=0&limit=%d",
			s.HTTPAddr(), a.ID, len(xssLoggerMockDriverStdout))
		resp, err := http.DefaultClient.Get(path)
		require.NoError(t, err)
		defer resp.Body.Close()

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, xssLoggerMockDriverStdout, string(buf))

		require.Equal(t, []string{"text/plain"}, resp.Header.Values("Content-Type"))
		require.Equal(t, []string{"nosniff"}, resp.Header.Values("X-Content-Type-Options"))
		require.Equal(t, []string{"1; mode=block"}, resp.Header.Values("X-XSS-Protection"))
		require.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"},
			resp.Header.Values("Content-Security-Policy"))
	})
}

func TestHTTP_FS_Cat(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("/v1/client/fs/cat/%s?path=alloc/logs/web.stdout.0", a.ID)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.FileCatRequest(respW, req)
		require.Nil(err)

		output, err := io.ReadAll(respW.Result().Body)
		require.Nil(err)
		require.EqualValues(defaultLoggerMockDriverStdout, output)
	})
}

// TestHTTP_FS_Cat_XSS asserts that the cat API is safe from XSS.
func TestHTTP_FS_Cat_XSS(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), xssLoggerMockDriver)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("%s/v1/client/fs/cat/%s?path=alloc/logs/web.stdout.0", s.HTTPAddr(), a.ID)
		resp, err := http.DefaultClient.Get(path)
		require.NoError(t, err)
		defer resp.Body.Close()

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, xssLoggerMockDriverStdout, string(buf))

		require.Equal(t, []string{"text/plain"}, resp.Header.Values("Content-Type"))
		require.Equal(t, []string{"nosniff"}, resp.Header.Values("X-Content-Type-Options"))
		require.Equal(t, []string{"1; mode=block"}, resp.Header.Values("X-XSS-Protection"))
		require.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"},
			resp.Header.Values("Content-Security-Policy"))
	})
}

func TestHTTP_FS_Stream_NoFollow(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := base64.StdEncoding.EncodeToString(
			[]byte(defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]))
		path := fmt.Sprintf("/v1/client/fs/stream/%s?path=alloc/logs/web.stdout.0&offset=%d&origin=end&follow=false",
			a.ID, offset)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := testutil.NewResponseRecorder()
		doneCh := make(chan struct{})
		go func() {
			_, err = s.Server.Stream(respW, req)
			require.Nil(err)
			close(doneCh)
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := io.ReadAll(respW)
			if err != nil {
				return false, err
			}

			out += string(output)
			return strings.Contains(out, expectation), fmt.Errorf("%q doesn't contain %q", out, expectation)
		}, func(err error) {
			t.Fatal(err)
		})

		select {
		case <-doneCh:
		case <-time.After(1 * time.Second):
			t.Fatal("should close but did not")
		}
	})
}

// TestHTTP_FS_Stream_NoFollow_XSS asserts that the stream API is safe from XSS.
func TestHTTP_FS_Stream_NoFollow_XSS(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), xssLoggerMockDriver)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("%s/v1/client/fs/stream/%s?path=alloc/logs/web.stdout.0&follow=false",
			s.HTTPAddr(), a.ID)
		resp, err := http.DefaultClient.Get(path)
		require.NoError(t, err)
		defer resp.Body.Close()

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		expected := `{"Data":"PHNjcmlwdD5hbGVydChkb2N1bWVudC5kb21haW4pOzwvc2NyaXB0Pg==","File":"alloc/logs/web.stdout.0","Offset":40}`
		require.Equal(t, expected, string(buf))
	})
}

func TestHTTP_FS_Stream_Follow(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := base64.StdEncoding.EncodeToString(
			[]byte(defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]))
		path := fmt.Sprintf("/v1/client/fs/stream/%s?path=alloc/logs/web.stdout.0&offset=%d&origin=end",
			a.ID, offset)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		doneCh := make(chan struct{})
		go func() {
			_, err = s.Server.Stream(respW, req)
			require.Nil(err)
			close(doneCh)
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := io.ReadAll(respW.Body)
			if err != nil {
				return false, err
			}

			out += string(output)
			return strings.Contains(out, expectation), fmt.Errorf("%q doesn't contain %q", out, expectation)
		}, func(err error) {
			t.Fatal(err)
		})

		select {
		case <-doneCh:
			t.Fatal("shouldn't close")
		case <-time.After(1 * time.Second):
		}
	})
}

func TestHTTP_FS_Logs(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]
		path := fmt.Sprintf("/v1/client/fs/logs/%s?type=stdout&task=web&offset=%d&origin=end&plain=true",
			a.ID, offset)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := testutil.NewResponseRecorder()
		go func() {
			_, err = s.Server.Logs(respW, req)
			require.Nil(err)
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := io.ReadAll(respW)
			if err != nil {
				return false, err
			}

			out += string(output)
			return out == expectation, fmt.Errorf("%q != %q", out, expectation)
		}, func(err error) {
			t.Fatal(err)
		})
	})
}

// TestHTTP_FS_Logs_XSS asserts that the logs endpoint always returns
// text/plain or application/json content regardless of whether the logs are
// HTML+Javascript or not.
func TestHTTP_FS_Logs_XSS(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), xssLoggerMockDriver)
		addAllocToClient(s, a, terminalClientAlloc)

		// Must make a "real" request to ensure Go's default content
		// type detection does not detect text/html
		path := fmt.Sprintf("%s/v1/client/fs/logs/%s?type=stdout&task=web&plain=true", s.HTTPAddr(), a.ID)
		resp, err := http.DefaultClient.Get(path)
		require.NoError(t, err)
		defer resp.Body.Close()

		buf, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, xssLoggerMockDriverStdout, string(buf))

		require.Equal(t, []string{"text/plain"}, resp.Header.Values("Content-Type"))
	})
}

func TestHTTP_FS_Logs_Follow(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]
		path := fmt.Sprintf("/v1/client/fs/logs/%s?type=stdout&task=web&offset=%d&origin=end&plain=true&follow=true",
			a.ID, offset)

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.Nil(err)
		respW := testutil.NewResponseRecorder()
		errCh := make(chan error, 1)
		go func() {
			_, err := s.Server.Logs(respW, req)
			errCh <- err
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := io.ReadAll(respW)
			if err != nil {
				return false, err
			}

			out += string(output)
			return out == expectation, fmt.Errorf("%q != %q", out, expectation)
		}, func(err error) {
			t.Fatal(err)
		})

		select {
		case err := <-errCh:
			t.Fatalf("shouldn't exit: %v", err)
		case <-time.After(1 * time.Second):
		}
	})
}

func TestHTTP_FS_Logs_PropagatesErrors(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		path := fmt.Sprintf("/v1/client/fs/logs/%s?type=stdout&task=web&offset=0&origin=end&plain=true",
			uuid.Generate())

		req, err := http.NewRequest(http.MethodGet, path, nil)
		require.NoError(t, err)
		respW := testutil.NewResponseRecorder()

		_, err = s.Server.Logs(respW, req)
		require.Error(t, err)

		_, ok := err.(HTTPCodedError)
		require.Truef(t, ok, "expected a coded error but found: %#+v", err)
	})
}
