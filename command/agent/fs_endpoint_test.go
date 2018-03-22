package agent

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

const (
	defaultLoggerMockDriverStdout = "Hello from the other side"
)

var (
	defaultLoggerMockDriver = map[string]interface{}{
		"run_for":       "2s",
		"stdout_string": defaultLoggerMockDriverStdout,
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
	require.Nil(state.UpsertJob(999, alloc.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{alloc}))

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
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/ls/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.DirectoryListRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())
	})
}

func TestHTTP_FS_Stat_MissingParams(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stat/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileStatRequest(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())
	})
}

func TestHTTP_FS_ReadAt_MissingParams(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/readat/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		require.NotNil(err)

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		require.NotNil(err)

		req, err = http.NewRequest("GET", "/v1/client/fs/readat/foo?path=/path/to/file", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileReadAtRequest(respW, req)
		require.NotNil(err)
	})
}

func TestHTTP_FS_Cat_MissingParams(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/cat/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.FileCatRequest(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/stat/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.FileCatRequest(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())
	})
}

func TestHTTP_FS_Stream_MissingParams(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/stream/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/stream/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.EqualError(err, fileNameNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/stream/foo?path=/path/to/file", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Stream(respW, req)
		require.Nil(err)
	})
}

func TestHTTP_FS_Logs_MissingParams(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("GET", "/v1/client/fs/logs/", nil)
		require.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.Logs(respW, req)
		require.EqualError(err, allocIDNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/logs/foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Logs(respW, req)
		require.EqualError(err, taskNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/logs/foo?task=foo", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Logs(respW, req)
		require.EqualError(err, logTypeNotPresentErr.Error())

		req, err = http.NewRequest("GET", "/v1/client/fs/logs/foo?task=foo&type=stdout", nil)
		require.Nil(err)
		respW = httptest.NewRecorder()

		_, err = s.Server.Logs(respW, req)
		require.Nil(err)
	})
}

func TestHTTP_FS_List(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		req, err := http.NewRequest("GET", "/v1/client/fs/ls/"+a.ID, nil)
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
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("/v1/client/fs/stat/%s?path=alloc/", a.ID)
		req, err := http.NewRequest("GET", path, nil)
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
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 1
		limit := 3
		expectation := defaultLoggerMockDriverStdout[offset : offset+limit]
		path := fmt.Sprintf("/v1/client/fs/readat/%s?path=alloc/logs/web.stdout.0&offset=%d&limit=%d",
			a.ID, offset, limit)

		req, err := http.NewRequest("GET", path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.FileReadAtRequest(respW, req)
		require.Nil(err)

		output, err := ioutil.ReadAll(respW.Result().Body)
		require.Nil(err)
		require.EqualValues(expectation, output)
	})
}

func TestHTTP_FS_Cat(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		path := fmt.Sprintf("/v1/client/fs/cat/%s?path=alloc/logs/web.stdout.0", a.ID)

		req, err := http.NewRequest("GET", path, nil)
		require.Nil(err)
		respW := httptest.NewRecorder()
		_, err = s.Server.FileCatRequest(respW, req)
		require.Nil(err)

		output, err := ioutil.ReadAll(respW.Result().Body)
		require.Nil(err)
		require.EqualValues(defaultLoggerMockDriverStdout, output)
	})
}

func TestHTTP_FS_Stream(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := base64.StdEncoding.EncodeToString(
			[]byte(defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]))
		path := fmt.Sprintf("/v1/client/fs/stream/%s?path=alloc/logs/web.stdout.0&offset=%d&origin=end",
			a.ID, offset)

		p, _ := io.Pipe()

		req, err := http.NewRequest("GET", path, p)
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
			output, err := ioutil.ReadAll(respW.Body)
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

		p.Close()
	})
}

func TestHTTP_FS_Logs(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]
		path := fmt.Sprintf("/v1/client/fs/logs/%s?type=stdout&task=web&offset=%d&origin=end&plain=true",
			a.ID, offset)

		p, _ := io.Pipe()
		req, err := http.NewRequest("GET", path, p)
		require.Nil(err)
		respW := httptest.NewRecorder()
		go func() {
			_, err = s.Server.Logs(respW, req)
			require.Nil(err)
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := ioutil.ReadAll(respW.Body)
			if err != nil {
				return false, err
			}

			out += string(output)
			return out == expectation, fmt.Errorf("%q != %q", out, expectation)
		}, func(err error) {
			t.Fatal(err)
		})

		p.Close()
	})
}

func TestHTTP_FS_Logs_Follow(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		a := mockFSAlloc(s.client.NodeID(), nil)
		addAllocToClient(s, a, terminalClientAlloc)

		offset := 4
		expectation := defaultLoggerMockDriverStdout[len(defaultLoggerMockDriverStdout)-offset:]
		path := fmt.Sprintf("/v1/client/fs/logs/%s?type=stdout&task=web&offset=%d&origin=end&plain=true&follow=true",
			a.ID, offset)

		p, _ := io.Pipe()
		req, err := http.NewRequest("GET", path, p)
		require.Nil(err)
		respW := httptest.NewRecorder()
		errCh := make(chan error)
		go func() {
			_, err := s.Server.Logs(respW, req)
			errCh <- err
		}()

		out := ""
		testutil.WaitForResult(func() (bool, error) {
			output, err := ioutil.ReadAll(respW.Body)
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

		p.Close()
	})
}
